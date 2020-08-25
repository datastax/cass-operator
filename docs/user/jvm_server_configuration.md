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
  serverVersion: 3.11.7
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

| Config Builder key | jvm.options property | Value type | Notes | 
| ------------------ | :-------------------:| :--------: | :---: |
| `additional-jvm-opts` | `JVM_OPTS` | Arbitrary JVM options passed to Cassandra on start up |
| `cassandra_ring_delay_ms` | `-Dcassandra.ring_delay_ms`| integer | Disabled by default |
| `log_gc` | `-Xloggc:/var/log/cassandra/gc.log` | boolean | Disabled by default |
| `thread_priority_policy_42` | `-XX:ThreadPriorityPolicy=42` | boolean | Enabled by default |
| `use_gc_log_file_rotation` | `-XX:+UseGCLogFileRotation` | boolean | Disabled by default |
| `initiating_heap_occupancy_percent` | `-XX:InitiatingHeapOccupancyPercent` | integer | Disabled by default. Can only be used when G1GC garbage collector is used. |
| `string_table_size` | `-XX:StringTableSize` | string | Defaults to 1000003 |
| `print_tenuring_distribution` | `-XX:+PrintTenuringDistribution` | boolean | Defaults to false |
| `cassandra_initial_token` | `-Dcassandra.initial_token` | string | Disabled by default |
| `resize_tlb` | `-XX:+ResizeTLAB` | boolean | Enabled by default |
| `cassandra_join_ring` | `-Dcassandra.join_ring` | boolean | Enabled by default |
| `jmx-remote-ssl-opts` | | string | Remote SSL options |
| `use_tlb` | `-XX:+UseTLAB` | boolean | Enabled by default |
| `perf_disable_shared_mem` | `-XX:+PerfDisableSharedMem` | boolean | Enabled by default |
| `cassandra_config_directory` | `-Dcassandra.config` | string | Disabled by default. Overriding this property may break the cluster. |
| `cms_wait_duration` | `-XX:CMSWaitDuration` | integer | Defaults to 10000. Can only be used when CMS garbage collector is used. |
| `cassandra_replace_address` | `-Dcassandra.replace_address` | string | Disabled by default. Overriding this property may break the cluster. |
| `heap_dump_on_out_of_memory_error` | `-XX:+HeapDumpOnOutOfMemoryError` | boolean | Enabled by default |
| `initial_heap_size` | `-Xms` | string | Disabled by default |
| `garbage_collector` | | string | Supported values are `CMS` and `G1GC`. Defaults to `G1GC`. |
| `gc_log_file_size` | `-XX:GCLogFileSize` | string | Disabled by default |
| `conc_gc_threads` | `-XX:ConcGCThreads` | integer | Disabled by default |
| `max_heap_size` | `-Xmx` | string | Disabled by default |
| `heap_size_young_generation` | `-Xmn` | string | Disabled by default |
| `max_gc_pause_millis` | `-XX:MaxGCPauseMillis` | integer | Defaults to `500`. Can only be used when G1 garbage collector is used. |
| `always_pre_touch` | `-XX:+AlwaysPreTouch` | boolean | Enabled by default |
| `unlock_commerical_features` | `-XX:+UnlockCommercialFeatures` | boolean | Disabled by default |
| `cassandra_disable_auth_caches_remote_configuration` | `-Dcassandra.disable_auth_caches_remote_configuration` | boolean | Disabled by default |
| `survivor_ratio` | `-XX:SurvivorRatio` | integer | Defaults to `8`. Can only be used when CMS garbage collector is used. |
| `g1r_set_updating_pause_time_percent` | `-XX:G1RSetUpdatingPauseTimePercent` | integer | Defaults to `5`. Can only be used when G1 garbage collector is used. |
| `java_net_prefer_ipv4_stack` | `-Djava.net.preferIPv4Stack=true` | boolean | Enabeled by default |
| `cassandra_load_ring_state` | `-Dcassandra.load_ring_state` | boolean | Enabled by default |
| `per_thread_stack_size` | `-Xss` | string | Defaults to `256k` |
| `use_biased_locking` | `-XX:-UseBiasedLocking` | boolean | Disabled by default |
| `cassandra_available_processors` | `-Dcassandra.available_processors` | integer | Disabled by default |
| `print_flss_statistics` | `-XX:PrintFLSStatistics=1` | boolean | Disabled by default |
| `print_heap_at_gc` | `-XX:+PrintHeapAtGC` | boolean | Disabled by default |
| `cassandra_write_survey` | `-Dcassandra.write_survey` | boolean | Disabled by default |
| `print_gc_application_stopped_time` | `-XX:+PrintGCApplicationStoppedTime` | boolean | Disabled by default |
| `print_promotion_failure` | `-XX:+PrintPromotionFailure` | boolean | Disabled by default |
| `parallel_gc_threads` | `-XX:ParallelGCThreads` | integer | Disabled by default. Can only be used when G1 garbage collector is used. |
| `jmx-connection-type` | | string | Possible values include `local-no-auth`, `remote-no-auth`, and `remote-dse-unified-auth`. Defaults to `local-no-auth` |
| `cassandra_force_default_indexing_page_size` | `-Dcassandra.force_default_indexing_page_size` | boolean | Disabled by default |
| `flight_recorder` | `-XX:+FlightRecorder` | boolean | Disabled by default |
| `cassandra_force_3_0_protocol_version` | `-Dcassandra.force_3_0_protocol_version=true` | boolean | Disabled by default |
| `cassandra_triggers_dir` | `-Dcassandra.triggers_dir` | string | Disabled by default |
| `cassandra_replay_list` | `-Dcassandra.replayList` | string | Disabled by default |
| `agent_lib_jdwp` | `-agentlib:jdwp=transport=dt_socket,server=y,suspend=n,address=1414"` | boolean | Disabled by default |
| `cms_initiating_occupancy_fraction` | `-XX:CMSInitiatingOccupancyFraction` | integer | Defaults to `75`. Can only be used when the CMS garbage collector is used. |
| `cassandra_metrics_reporter_config_file` | `-Dcassandra.metricsReporterConfigFile` | string | Disabled by default |
| `max_tenuring_threshold` | `-XX:MaxTenuringThreshold` | integer | Defaults to `1`. Can only be used when the CMS garbage collector is used. |
| `number_of_gc_log_files` | `-XX:NumberOfGCLogFiles` | integer | Disabled by default. Can only be used when the G1 garbage collector is used. |
|
| `print_gc_details` | `-XX:+PrintGCDetails` | boolean | Disabled by default |
| `enable_assertions` | `-ea` | boolean | Enabled by default |
| `use_thread_priorities` | `-XX:+UseThreadPriorities` | boolean | Enabled by default |

### Cassandra 4.0
To configure the server JVM with Cassandra 4.0 you need to use the `jvm-server-options` key which corresonds to the `CASSANDRA_CONF/jvm-server.options` file.

Here is a brief example showing it is used in a YAML manifest:

```yaml
apiVersion: cassandra.datastax.com/v1beta1
kind: CassandraDatacenter
metadata:
  name: dc1
spec:
  clusterName: cluster1
  serverType: cassandra
  serverVersion: 4.0
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
    jvm-server-options:
      initial_heap_size: "800m"
      max_heap_size: "800m"
```

The following table lists the supports keys that can appear under `jvm-server-options` along with their corresponding property names in `jvm-server.options`.

| Config Builder key | jvm.options property | Value type | Notes | 
| ------------------ | :-------------------:| :--------: | :---: |
| `additional-jvm-opts` | `JVM_OPTS` | Arbitrary JVM options passed to Cassandra on start up |
| `jmx-connection-type` | | string | Possible values include `local-no-auth`, `remote-no-auth`. Defaults to `local-no-auth` |
| `unlock-diagnostic-vm-options` | `-XX:+UnlockDiagnosticVMOption` | boolean | Enabled by default |
| `cassandra_available_processors` | `-Dcassandra.available_processors` | integer | Disabled by default |
| `cassandra_config_directory` | `-Dcassandra.config` | string | Disabled by default. Overriding this property may break the cluster. |
| `cassandra_initial_token` | `-Dcassandra.initial_token` | string | Disabled by default |
| `cassandra_join_ring` | `-Dcassandra.join_ring` | boolean | Enabled by default |
| `cassandra_load_ring_state` | `-Dcassandra.load_ring_state` | boolean | Enabled by default |
| `cassandra_metrics_reporter_config_file` | `-Dcassandra.metricsReporterConfigFile` | string | Disabled by default |
| `cassandra_replace_address` | `-Dcassandra.replace_address` | string | Disabled by default. Overriding this property may break the cluster. |
| `cassandra_ring_delay_ms` | `-Dcassandra.ring_delay_ms`| integer | Disabled by default |
| `cassandra_triggers_dir` | `-Dcassandra.triggers_dir` | string | Disabled by default |
| `cassandra_write_survey` | `-Dcassandra.write_survey` | boolean | Disabled by default |
| `cassandra_disable_auth_caches_remote_configuration` | `-Dcassandra.disable_auth_caches_remote_configuration` | boolean | Disabled by default |
| `cassandra_force_default_indexing_page_size` | `-Dcassandra.force_default_indexing_page_size` | boolean | Disabled by default |
| `cassandra_max_hint_ttl` | `-Dcassandra.maxHintTTL` | string | Disabled by default |
| `enable_assertions` | `-ea` | boolean | Enabled by default |
| `use_thread_priorities` | `-XX:+UseThreadPriorities` | boolean | Enabled by default |
| `heap_dump_on_out_of_memory_error` | `-XX:+HeapDumpOnOutOfMemoryError` | boolean | Enabled by default |
| `per_thread_stack_size` | `-Xss` | string | Defaults to `256k` |
| `string_table_size` | `-XX:StringTableSize` | string | Defaults to 1000003 |
| `always_pre_touch` | `-XX:+AlwaysPreTouch` | boolean | Enabled by default |
| `use_tlb` | `-XX:+UseTLAB` | boolean | Enabled by default |
| `resize_tlb` | `-XX:+ResizeTLAB` | boolean | Enabled by default |
| `use_numa` | `-XX:+UseNUMA` | boolean | Enabled by default |
| `perf_disable_shared_mem` | `-XX:+PerfDisableSharedMem` | boolean | Enabled by default |
| `java_net_prefer_ipv4_stack` | `-Djava.net.preferIPv4Stack=true` | boolean | Enabeled by default |
| `page-align-direct-memory` | `-Dsun.nio.PageAlignDirectMemory=true` | boolean | Enabled by default |
| `restrict-contended` | `-XX:-RestrictContended` | boolean | Enabled by default |
| `guaranteed-safepoint-interval` | `-XX:GuaranteedSafepointInterval` | string | Defaults to `300000` |
| `use-biased-locking` | `-XX:-UseBiasedLocking` | boolean | Enabled by default |
| `debug-non-safepoints` | `-XX:+DebugNonSafepoints` | boolean | Enabled by default |
| `preserve-frame-pointer` | `-XX:+PreserveFramePointer` | boolean | Enabled by default |
| `unlock_commercial_features` | `-XX:+UnlockCommercialFeatures` | boolean | Disabled by default |
| `flight_recorder` | `-XX:+FlightRecorder` | boolean | Disabled by default |
| `agent_lib_jdwp` | `-agentlib:jdwp=transport=dt_socket,server=y,suspend=n,address=1414"` | boolean | Disabled by default |
| `log_compilation` | `-XX:+LogCompilation` | boolean | Disabled by default |
| `initial_heap_size` | `-Xms` | string | Disabled by default |
| `max_heap_size` | `-Xmx` | string | Disabled by default |
| `jdk_nio_maxcachedbuffersize` | `-Djdk.nio.maxCachedBufferSize` | integer | Defaults to `1048576` |
| `cassandra_expiration_date_overflow_policy` | `-Dcassandra.expiration_date_overflow_policy` | string | Possible values include `REJECT`, `CAP`, `CAP_NOWARN` |
| `io_netty_eventloop_maxpendingtasks` | `-Dio.netty.eventLoop.maxPendingTasks` | integer | Defaults to `65536` |
| `crash_on_out_of_memory_error` | `-XX:+CrashOnOutOfMemoryError` | boolean | Disabled by default. Requires `exit_on_out_of_memory_error` to be disabled. |
| `print_heap_histogram_on_out_of_memory_error` | `-Dcassandra.printHeapHistogramOnOutOfMemoryError` | boolean | Disabled by default |
| `exit_on_out_of_memory_error` | `-XX:+ExitOnOutOfMemoryError` | boolean | Disabled by default |