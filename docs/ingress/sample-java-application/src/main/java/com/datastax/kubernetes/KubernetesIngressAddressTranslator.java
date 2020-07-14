package com.datastax.kubernetes;

import com.datastax.oss.driver.api.core.addresstranslation.AddressTranslator;
import com.datastax.oss.driver.api.core.context.DriverContext;
import edu.umd.cs.findbugs.annotations.NonNull;

import java.net.InetSocketAddress;

public class KubernetesIngressAddressTranslator implements AddressTranslator {
  private DriverContext driverContext;

  public KubernetesIngressAddressTranslator(DriverContext driverContext) {
    this.driverContext = driverContext;
  }

  @NonNull
  @Override
  public InetSocketAddress translate(@NonNull InetSocketAddress address) {
    String ingressAddress = driverContext.getConfig().getDefaultProfile().getString(KubernetesOption.INGRESS_ADDRESS);
    int ingressPort = driverContext.getConfig().getDefaultProfile().getInt(KubernetesOption.INGRESS_PORT);

    return new InetSocketAddress(ingressAddress, ingressPort);
  }

  @Override
  public void close() {
    // NOOP
  }
}
