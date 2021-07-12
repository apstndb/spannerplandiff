# spannerplandiff

Show Cloud Spanner query plans and diff between optimizer versions(EXPERIMENTAL).

## Usage

```
Usage:
  spannerplandiff [OPTIONS]

Application Options:
      --before=            Before query optimizer version
      --after=             After query optimizer version
  -p, --project=           (required) ID of the project. [$CLOUDSDK_CORE_PROJECT]
  -i, --instance=          (required) ID of the instance. [$CLOUDSDK_SPANNER_INSTANCE]
  -d, --database=          (required) ID of the database. [$DATABASE_ID]
      --format=[json|yaml] Output format. (default: yaml)
      --sql=               SQL query text; exclusive with --sql-file.
  -f, --sql-file=          File name contains SQL query; exclusive with --sql
  -o, --output=            Output file
      --log-grpc           Show gRPC logs
      --error-on-diff      Return exit code 1 when plans are differ

Help Options:
  -h, --help               Show this help message
```

```
$ spannerplandiff --error-on-diff --before=1 --after=2 --sql='SELECT SongName FROM Songs WHERE REGEXP_CONTAINS(SongName, "^a.*")' -o result.txtar
exit status 1
$ echo $?
1
$ cat retult.txtar 
-- 1.plan.yaml --
plannodes:
(ellipsis...)
-- 2.plan.yaml --
plannodes:
(ellipsis...)
-- diff_in_proto.txt --
  (*spanner.QueryPlan)(Inverse(protocmp.Transform, protocmp.Message{
        "@type": s"google.spanner.v1.QueryPlan",
        "plan_nodes": []protocmp.Message{
                {"@type": s"google.spanner.v1.PlanNode", "child_links": []protocmp.Message{{"@type": s"google.spanner.v1.PlanNode.ChildLink", "child_index": int32(1)}, {"@type": s"google.spanner.v1.PlanNode.ChildLink", "child_index": int32(11), "type": string("Split Range")}}, "display_name": string("Distributed Union"), "kind": s"RELATIONAL", ...},
                {"@type": s"google.spanner.v1.PlanNode", "child_links": []protocmp.Message{{"@type": s"google.spanner.v1.PlanNode.ChildLink", "child_index": int32(2)}}, "display_name": string("Distributed Union"), "index": int32(1), ...},
                {"@type": s"google.spanner.v1.PlanNode", "child_links": []protocmp.Message{{"@type": s"google.spanner.v1.PlanNode.ChildLink", "child_index": int32(3)}, {"@type": s"google.spanner.v1.PlanNode.ChildLink", "child_index": int32(10)}}, "display_name": string("Serialize Result"), "index": int32(2), ...},
                {
                        "@type": s"google.spanner.v1.PlanNode",
                        "child_links": []protocmp.Message{
                                {"@type": s"google.spanner.v1.PlanNode.ChildLink", "child_index": int32(4)},
                                {
                                        "@type":       s"google.spanner.v1.PlanNode.ChildLink",
                                        "child_index": int32(9),
-                                       "type":        string("Residual Condition"),
+                                       "type":        string("Seek Condition"),
                                },
                        },
                        "display_name": string("Filter Scan"),
                        "index":        int32(3),
                        "kind":         s"RELATIONAL",
                },
                {
                        ... // 3 identical entries
                        "index": int32(4),
                        "kind":  s"RELATIONAL",
                        "metadata": protocmp.Message{
                                "@type": s"google.protobuf.Struct",
                                "fields": map[string]protocmp.Message{
-                                       "Full scan": s`{string_value:"true"}`,
                                        "scan_target": {
                                                "@type":        s"google.protobuf.Value",
-                                               "string_value": string("SongsBySingerAlbumSongNameDesc"),
+                                               "string_value": string("SongsBySongName"),
                                        },
                                        "scan_type": {"@type": s"google.protobuf.Value", "string_value": string("IndexScan")},
                                },
                        },
                },
                {"@type": s"google.spanner.v1.PlanNode", "display_name": string("Reference"), "index": int32(5), "kind": s"SCALAR", ...},
                {
                        ... // 3 identical entries
                        "index": int32(6),
                        "kind":  s"SCALAR",
                        "short_representation": protocmp.Message{
                                "@type":       s"google.spanner.v1.PlanNode.ShortRepresentation",
-                               "description": string("REGEXP_CONTAINS($SongName, '^a.*')"),
+                               "description": string("STARTS_WITH($SongName, 'a')"),
                        },
                },
                {"@type": s"google.spanner.v1.PlanNode", "display_name": string("Reference"), "index": int32(7), "kind": s"SCALAR", ...},
                {
                        ... // 2 identical entries
                        "index": int32(8),
                        "kind":  s"SCALAR",
                        "short_representation": protocmp.Message{
                                "@type":       s"google.spanner.v1.PlanNode.ShortRepresentation",
-                               "description": string("'^a.*'"),
+                               "description": string("'a'"),
                        },
                },
                {
                        ... // 3 identical entries
                        "index": int32(9),
                        "kind":  s"SCALAR",
                        "short_representation": protocmp.Message{
                                "@type":       s"google.spanner.v1.PlanNode.ShortRepresentation",
-                               "description": string("REGEXP_CONTAINS($SongName, '^a.*')"),
+                               "description": string("STARTS_WITH($SongName, 'a')"),
                        },
                },
                {"@type": s"google.spanner.v1.PlanNode", "display_name": string("Reference"), "index": int32(10), "kind": s"SCALAR", ...},
+               s`{index:11  kind:SCALAR  display_name:"Function"  child_links:[{child_index:12}  {child_index:13}]  short_representation:{description:"STARTS_WITH($SongName, 'a')"}}`,
+               s`{index:12  kind:SCALAR  display_name:"Reference"  short_representation:{description:"$SongName"}}`,
                {
                        "@type":        s"google.spanner.v1.PlanNode",
                        "display_name": string("Constant"),
-                       "index":        int32(11),
+                       "index":        int32(13),
                        "kind":         s"SCALAR",
                        "short_representation": protocmp.Message{
                                "@type":       s"google.spanner.v1.PlanNode.ShortRepresentation",
-                               "description": string("true"),
+                               "description": string("'a'"),
                        },
                },
        },
  }))
```