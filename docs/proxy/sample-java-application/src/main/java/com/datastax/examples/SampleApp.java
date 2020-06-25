package com.datastax.examples;

import com.datastax.oss.driver.api.core.CqlSession;
import com.datastax.oss.driver.api.core.config.DriverConfigLoader;
import com.datastax.oss.driver.api.core.cql.ResultSet;
import com.datastax.oss.driver.api.core.metadata.Node;
import com.datastax.oss.driver.internal.core.metadata.SniEndPoint;

import java.net.InetSocketAddress;

public class SampleApp {
  public static void main( String[] args ) throws Exception {
    SampleApp app = new SampleApp();
    app.run();
  }

  public void run() throws Exception {
    CqlSession session = getLoadBalancedSession();

    System.out.println("Discovered Nodes");
    for (Node n : session.getMetadata().getNodes().values()) {
      System.out.println(String.format("%s:%s:%s", n.getDatacenter(), n.getRack(), n.getHostId()));
    }
    System.out.println();

    ResultSet rs = session.execute("SELECT data_center, rack, host_id, release_version FROM system.local");
    Node n = rs.getExecutionInfo().getCoordinator();
    System.out.println(String.format("Coordinator: %s:%s:%s", n.getDatacenter(), n.getRack(), n.getHostId()));
    rs.forEach(row -> {
      System.out.println(row.getFormattedContents());
    });
    System.out.println();

    rs = session.execute("SELECT data_center, rack, host_id, release_version FROM system.peers");
    n = rs.getExecutionInfo().getCoordinator();
    System.out.println(String.format("Coordinator: %s:%s:%s", n.getDatacenter(), n.getRack(), n.getHostId()));
    rs.forEach(row -> {
      System.out.println(row.getFormattedContents());
    });

    session.close();
  }

  private CqlSession getLoadBalancedSession() {
    return CqlSession.builder()
      .withConfigLoader(DriverConfigLoader.fromClasspath("load-balanced.conf"))
      .build();
  }

  private CqlSession getMtlsLoadBalancedSession() {
    return CqlSession.builder()
      .withConfigLoader(DriverConfigLoader.fromClasspath("mtls-load-balanced.conf"))
      .build();
  }

  private CqlSession getMtlsSniSession() {
    // Ingress address
    InetSocketAddress ingressAddress = new InetSocketAddress("traefik.k3s.local", 9042);

    // Endpoint (contact point)
    SniEndPoint endPoint = new SniEndPoint(ingressAddress, "ec448e83-8b83-407b-b342-13ce0250001c");

    return CqlSession.builder()
      .withConfigLoader(DriverConfigLoader.fromClasspath("mtls-sni.conf"))
      .build();
  }
}
