This page documents the keys for the JVM server options across the various supported versions.

### Cassandra 3.11
To configure the server JVM with Cassandra 3.11 you need to use the `jvm-options` key which corresonds to the `CASSANDRA_CONF/jvm.options` file.

Here is a brief example showing it is used in a YAML manifest:

```yaml
apiVersion: cassandra.datastax.com/v1beta1
kind: CassandraDatacenter
metadata:
  name: dc1
spec:
  clusterName: cluster1
  serverType: cassandra
  serverVersion: 3.11.6
  size: 3
  storageConfig:
      cassandraDataVolumeClaimSpec:
        storageClassName: server-storage
        accessModes:
          - ReadWriteOnce
        resources:
          requests:
            storage: 1Gi
  racks:
    - name: r1
    - name: r2
    - name: r3
  config:
    jvm-options:
      initial_heap_size: "800m"
      max_heap_size: "800m"
```
The following table lists the supports keys that can appear under `jvm-options` along with their corresponding property names in `jvm.options`.

| Config Builder key | jvm.options property | value type | notes | 
| ------------------ | :-------------------:| :--------: | :---: |
| `cassandra_ring_delay_ms` | `-Dcassandra.ring_delay_ms`| integer | Disabled by default |
| `log_gc` | `-Xloggc:/var/log/cassandra/gc.log` | boolean | Disabled by default |
| `thread_priority_policy_42` | `-XX:ThreadPriorityPolicy=42` | boolean | Enabled by default |
| use_gc_log_file_rotation | `-XX:+UseGCLogFileRotation` | boolean | Disabled by default |
| `initiating_heap_occupancy_percent` | `-XX:InitiatingHeapOccupancyPercent` | integer | Disabled by default. Can only be used when G1GC garbage collector is used. |
| `string_table_size` | `-XX:StringTableSize` | string | Defaults to 1000003 |
| `print_tenuring_distribution` | `-XX:+PrintTenuringDistribution` | boolean | Defaults to false |
| `cassandra_initial_token` | `-Dcassandra.initial_token` | string | Disabled by default |
| `resize_tlb` | `-XX:+ResizeTLAB` | boolean | Enabled by default |
| `cassandra_join_ring` | `-Dcassandra.join_ring` | boolean | Enabled by default |
| `use_tlb` | `-XX:+UseTLAB` | boolean | Enabled by default |
| `perf_disable_shared_mem` | `-XX:+PerfDisableSharedMem` | boolean | Enabled by default |
| `cassandra_config_directory` | `-Dcassandra.config` | string | Disabled by default |
| `cms_wait_duration` | `-XX:CMSWaitDuration` | integer | Defaults to 10000. Can only be used when CMS garbage collector is used. |
| `cassandra_replace_address` | `-Dcassandra.replace_address` | string | Disabled by default |
| `heap_dump_on_out_of_memory_error` | `-XX:+HeapDumpOnOutOfMemoryError` | boolean | Enabled by default |
| `initial_heap_size` | `-Xms` | string | Disabled by default |
| `garbage_collector` | | string | Supported values are `CMS` and `G1GC`. Defaults to `G1GC`. |
| `gc_log_file_size` | `-XX:GCLogFileSize` | string | Disabled by default |
| `conc_gc_threads` | `-XX:ConcGCThreads` | integer | Disabled by default |
| `max_heap_size` | `-Xmx` | string | Disabled by default |
| `heap_size_young_generation` | `-Xmn` | string | Disabled by default |
| `max_gc_pause_millis` | `-XX:MaxGCPauseMillis` | integer | Defaults to `500`. Can only be used when G1GC garbage collector is used. |