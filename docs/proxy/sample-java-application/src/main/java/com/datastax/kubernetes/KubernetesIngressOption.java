package com.datastax.kubernetes;

import com.datastax.oss.driver.api.core.config.DriverOption;
import edu.umd.cs.findbugs.annotations.NonNull;

public enum KubernetesIngressOption implements DriverOption {
  INGRESS_ADDRESS("advanced.k8s.ingress.address"),
  INGRESS_PORT("advanced.k8s.ingress.port");

  private final String path;

  KubernetesIngressOption(String path) {
    this.path = path;
  }

  @Override
  @NonNull
  public String getPath() {
    return path;
  }
}
