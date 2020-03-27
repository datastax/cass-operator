# DSE Configuration

## Overview

The configuration files for the DseDatacenter can be adjusted using the Config field in the DseDatacenter.Spec. The fields can be directly specified as yaml:

```yaml
  config:
    cassandra-yaml:
      authenticator: AllowAllAuthenticator
      batch_size_fail_threshold_in_kb: 1280
```

The valid fields for each file differ according to DSE version.

## Available Config Files

The following files are supported, each one of them can be parameterized by using the following ids:

|ID:                          |Path:                                                     |
|:----------------------------|---------------------------------------------------------:|
|10-statsd-conf               |resources/dse/collectd/etc/collectd/10-statsd.conf        |
|10-write-graphite-conf       |resources/dse/collectd/etc/collectd/10-write-graphite.conf|
|10-write-prom-conf           |resources/dse/collectd/etc/collectd/10-write-prom.conf    |
|cassandra-env-sh             |resources/cassandra/conf/cassandra-env.sh                 |
|cassandra-rackdc-properties  |resources/cassandra/conf/cassandra-rackdc.properties      |
|cassandra-yaml               |resources/cassandra/conf/cassandra.yaml                   |
|dse-default                  |resources/dse/conf/dse.default                            |
|dse-yaml                     |resources/dse/conf/dse.yaml                               |
|jvm11-server-options         |resources/cassandra/conf/jvm11-server.options             |
|jvm8-server-options          |resources/cassandra/conf/jvm8-server.options              |
|jvm-server-options           |resources/cassandra/conf/jvm-server.options               |
|logback-xml                  |resources/cassandra/conf/logback.xml                      |

## Field names

The format and naming of the fields for each config file is identical to the names used in Datastax LifeCycle Manager.

## Field values and examples

The following data types are accepted:

- Booleans:          true, false
- Integers:          0, 10, 200
- Floating Point:    2.34
- Strings:           AllowAllAuthenticator

## Grouping of Fields

Some fields are nested inside of other fields. To specify these values, include the name of the grouping at each level. This example uses the "node_health_options" group:

```yaml
  config:
    dse-yaml:
      node_health_options:
        refresh_rate_ms: 50000
        uptime_ramp_up_period_seconds: 10800
        dropped_mutation_window_minutes: 30
```

## Detailed file information

There are in-depth write-ups for the cassandra-yaml and dse-yaml files available in the Datastax documentation:

https://docs.datastax.com/en/dse/6.7/dse-admin/datastax_enterprise/config/configCassandra_yaml.html

https://docs.datastax.com/en/dse/6.7/dse-admin/datastax_enterprise/config/configDseYaml.html

## cassandra-yaml Configs

### cluster_name

Note that the cluster_name field should not be explicitly set in the cassandra-yaml config. The operator will automatically use the clusterName field from the DseDatacenter resource.

### Example: Disable Authentication

Note that password-based authentication is enabled by default.

```yaml
  config:
    cassandra-yaml:
      authenticator: AllowAllAuthenticator
```


## Example: Enable Node-To-Node Encryption

```yaml
  config:
    cassandra-yaml:
      server_encryption_options:
        internode_encryption: all
        keystore: resources/dse/conf/.keystore
        keystore_password: cassandra
        truststore: resources/dse/conf/.truststore
        truststore_password: cassandra
```


## Example: Enable Client-To-Node Encryption

```yaml
  config:
    cassandra-yaml:
      client_encryption_options:
        enabled: true
        optional: false
        keystore: resources/dse/conf/.keystore
        keystore_password: cassandra
```


## Example: DSE Node Health Options dse-yaml Config

```yaml
  config:
    dse-yaml:
      node_health_options:
        refresh_rate_ms: 50000
        uptime_ramp_up_period_seconds: 10800
        dropped_mutation_window_minutes: 30
```


## Example: DSE System Info Encryption dse-yaml Config

```yaml
  config:
    dse-yaml:
      system_info_encryption:
        enabled: true
        secret_key_strength: 256
```


## Commonly Set Fields

The following example lists some commonly set fields:

```yaml
  config:
    10-statsd-conf:
      enabled: true
    10-write-prom-conf:
      enabled: true
      ports: 9103
    cassandra-env-sh:
      heap-dump-dir: /tmp
    cassandra-yaml:
      commit_failure_policy: stop
      disk_optimization_strategy: ssd
      disk_failure_policy: stop
      enable_user_defined_functions: false
      enable_scripted_user_defined_functions: false
      enable_user_defined_functions_threads: true
      num_tokens: 128
      user_defined_function_warn_micros: 500
      user_defined_function_fail_micros: 10000
      user_defined_function_warn_heap_mb: 200
      user_defined_function_fail_heap_mb: 500
      user_function_timeout_policy: die
    dse-default:
      wait-for-start: 5
      wait-for-stop: 120
      wait-for-start-sleep: 10
      wait-for-stop-sleep: 5
    dse-yaml:
      authorization_options:
        enabled: true
        allow_row_level_security: true
        transitional_mode: strict
      role_management_options:
        mode: internal
    jvm11-server-options:
      io_netty_try_reflection_set_accessible: false
    jvm8-server-options:
      log_gc: true
    jvm-server-options:
      per_thread_stack_size: 512k
      flight_recorder: true
      max_heap_size: 2g
    logback-xml:
      root-log-level: DEBUG
      systemlog-appender-level:DEBUG
```
