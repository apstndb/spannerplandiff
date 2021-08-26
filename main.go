package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"

	"cloud.google.com/go/spanner"
	"github.com/apstndb/protoyaml"
	"github.com/google/go-cmp/cmp"
	grpc_zap "github.com/grpc-ecosystem/go-grpc-middleware/logging/zap"
	"github.com/jessevdk/go-flags"
	"go.uber.org/zap"
	"golang.org/x/tools/txtar"
	"google.golang.org/api/option"
	spannerpb "google.golang.org/genproto/googleapis/spanner/v1"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/testing/protocmp"
)

var errDiff = errors.New("success but need to return exit code 1")
var errHelp = errors.New("help is printed")

func main() {
	if err := run(context.Background()); err == errHelp {
		os.Exit(-1)
	} else if err == errDiff {
		os.Exit(1)
	} else if err != nil {
		log.Fatalln(err)
	}
}

type opts struct {
	Before      string `long:"before" description:"Before query optimizer version" required:"true"`
	After       string `long:"after" description:"After query optimizer version" required:"true"`
	Project     string `long:"project" short:"p" description:"(required) ID of the project." required:"true" env:"CLOUDSDK_CORE_PROJECT"`
	Instance    string `long:"instance" short:"i" description:"(required) ID of the instance." required:"true" env:"CLOUDSDK_SPANNER_INSTANCE"`
	Database    string `long:"database" short:"d" description:"(required) ID of the database." required:"true" env:"DATABASE_ID"`
	Format      string `long:"format" description:"Output format." default:"yaml" choice:"json" choice:"yaml"`
	Sql         string `long:"sql" description:"SQL query text; exclusive with --sql-file."`
	SqlFile     string `long:"sql-file" short:"f" description:"File name contains SQL query; exclusive with --sql"`
	Output      string `long:"output" short:"o" description:"Output file"`
	LogGrpc     bool   `long:"log-grpc" description:"Show gRPC logs"`
	ErrorOnDiff bool   `long:"error-on-diff" description:"Return exit code 1 when plans are differ"`
}

func processFlags() (o opts, err error) {
	flagParser := flags.NewParser(&o, flags.Default)
	defer func() {
		if err == nil {
			return
		}
		if e, ok := err.(*flags.Error); ok && e.Type == flags.ErrHelp {
			return
		}
		log.Print(err)
		flagParser.WriteHelp(os.Stderr)
	}()
	_, err = flagParser.Parse()
	if err != nil {
		return o, err
	}
	if o.SqlFile != "" && o.Sql != "" {
		return o, errors.New("--sql-file and --sql are mutually exclusive")
	}
	if o.SqlFile == "" && o.Sql == "" {
		return o, errors.New("--sql-file or --sql is required")
	}

	return o, nil
}

func run(ctx context.Context) error {
	o, err := processFlags()
	if err != nil {
		// err is already printed in processFlags()
		return errHelp
	}
	var sql string
	if o.SqlFile != "" {
		b, err := os.ReadFile(o.SqlFile)
		if err != nil {
			return err
		}
		sql = string(b)
	} else {
		sql = o.Sql
	}

	client, err := newClient(ctx, o.Project, o.Instance, o.Database, o.LogGrpc)
	if err != nil {
		return err
	}
	defer client.Close()

	plans := make(map[string]*spannerpb.QueryPlan)
	for _, version := range []string{o.Before, o.After} {
		err := func() error {
			// AnalyzeQuery doesn't have WithOptions variant so use QueryWithOptions
			mode := spannerpb.ExecuteSqlRequest_PLAN
			it := client.Single().QueryWithOptions(ctx, spanner.NewStatement(sql), spanner.QueryOptions{
				Mode: &mode,
				Options: &spannerpb.ExecuteSqlRequest_QueryOptions{
					OptimizerVersion: version,
				},
			})
			defer it.Stop()
			// Consume for get plan
			if err := it.Do(func(r *spanner.Row) error { return nil }); err != nil {
				return err
			}
			plans[version] = it.QueryPlan
			return nil
		}()
		if err != nil {
			return err
		}
	}

	var files []txtar.File
	for _, name := range []string{o.Before, o.After} {
		plan := plans[name]
		var b []byte
		switch o.Format {
		case "json":
			b, err = protojson.Marshal(plan)
			if err != nil {
				return err
			}
		case "yaml":
			b, err = protoyaml.Marshal(plan)
			if err != nil {
				return err
			}
		}
		files = append(files, txtar.File{
			Name: fmt.Sprintf("%s.plan.%s", name, o.Format),
			Data: b,
		})
	}

	beforePlan := plans[o.Before]
	afterPlan := plans[o.After]
	samePlans := cmp.Equal(beforePlan, afterPlan, protocmp.Transform())
	if !samePlans {
		files = append(files, txtar.File{Name: "diff_in_proto.txt", Data: []byte(cmp.Diff(beforePlan, afterPlan, protocmp.Transform()))})
	}
	var writer io.Writer
	if o.Output != "" {
		f, err := os.Create(o.Output)
		if err != nil {
			return err
		}
		defer f.Close()
		writer = f
	} else {
		writer = os.Stdout
	}

	writer.Write(txtar.Format(&txtar.Archive{
		Comment: nil,
		Files:   files,
	}))
	if o.ErrorOnDiff && !samePlans {
		return errDiff
	}
	return nil
}

func newClient(ctx context.Context, project, instance, database string, logGrpc bool) (*spanner.Client, error) {
	name := fmt.Sprintf("projects/%s/instances/%s/databases/%s", project, instance, database)

	var copts []option.ClientOption
	if logGrpc {
		copts = logGrpcClientOptions()
	}

	return spanner.NewClientWithConfig(ctx, name, spanner.ClientConfig{
		SessionPoolConfig: spanner.SessionPoolConfig{
			MaxOpened:           1,
			MinOpened:           1,
			WriteSessions:       0,
			TrackSessionHandles: true,
		},
	}, copts...)
}

func logGrpcClientOptions() []option.ClientOption {
	zapDevelopmentConfig := zap.NewDevelopmentConfig()
	zapDevelopmentConfig.DisableCaller = true
	zapLogger, _ := zapDevelopmentConfig.Build(zap.Fields())

	return []option.ClientOption{
		option.WithGRPCDialOption(grpc.WithChainUnaryInterceptor(
			grpc_zap.PayloadUnaryClientInterceptor(zapLogger, func(ctx context.Context, fullMethodName string) bool {
				return true
			}),
			grpc_zap.UnaryClientInterceptor(zapLogger),
		)),
		option.WithGRPCDialOption(grpc.WithChainStreamInterceptor(
			grpc_zap.PayloadStreamClientInterceptor(zapLogger, func(ctx context.Context, fullMethodName string) bool {
				return true
			}),
			grpc_zap.StreamClientInterceptor(zapLogger),
		)),
	}
}
