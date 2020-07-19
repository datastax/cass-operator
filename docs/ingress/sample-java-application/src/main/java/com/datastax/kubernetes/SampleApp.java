package com.datastax.kubernetes;

import com.datastax.oss.driver.api.core.CqlSession;
import com.datastax.oss.driver.api.core.config.DefaultDriverOption;
import com.datastax.oss.driver.api.core.config.DriverConfigLoader;
import com.datastax.oss.driver.api.core.config.DriverExecutionProfile;
import com.datastax.oss.driver.api.core.cql.ResultSet;
import com.datastax.oss.driver.api.core.metadata.EndPoint;
import com.datastax.oss.driver.api.core.metadata.Node;
import com.datastax.oss.driver.internal.core.metadata.SniEndPoint;
import com.datastax.oss.driver.internal.core.ssl.SniSslEngineFactory;

import java.io.InputStream;
import java.net.InetSocketAddress;
import java.nio.file.Files;
import java.nio.file.Paths;
import java.security.KeyStore;
import java.security.SecureRandom;
import java.util.HashSet;
import java.util.List;
import java.util.Set;

import javax.net.ssl.KeyManagerFactory;
import javax.net.ssl.SSLContext;
import javax.net.ssl.TrustManagerFactory;

import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

public class SampleApp {
  final static Logger logger = LoggerFactory.getLogger(SampleApp.class);

  public static void main( String[] args ) throws Exception {
    if (args.length < 1) {
      System.err.println("Missing connection type argument. Possible options:");
      printConnectionTypes();
    } else {
      CqlSession session = null;

      if (args[0].equals("direct")) {
        session = CqlSession.builder()
          .withConfigLoader(DriverConfigLoader.fromClasspath("direct.conf"))
          .build();
      } else if (args[0].equals("ingress")) {
        session = CqlSession.builder()
          .withConfigLoader(DriverConfigLoader.fromClasspath("ingress.conf"))
          .build();
      } else if (args[0].equals("sni-ingress")) {
        DriverConfigLoader configLoader = DriverConfigLoader.fromClasspath("sni-ingress.conf");
        DriverExecutionProfile config = configLoader.getInitialConfig().getDefaultProfile(); 

        String ingressAddress = config.getString(KubernetesOption.INGRESS_ADDRESS);
        int ingressPort = config.getInt(KubernetesOption.INGRESS_PORT);
        InetSocketAddress cloudProxyAddress = new InetSocketAddress(ingressAddress, ingressPort);

        Set<EndPoint> builtEndpoints = new HashSet<>();
        List<String> configEndpoints = config.getStringList(KubernetesOption.ENDPOINTS);
        for (String configEndpoint : configEndpoints) {
          builtEndpoints.add(new SniEndPoint(cloudProxyAddress, configEndpoint));
        }

        SSLContext sslContext = buildContext(config);

        session = CqlSession.builder()
          .withConfigLoader(configLoader)
          .withCloudProxyAddress(cloudProxyAddress)
          .withSslEngineFactory(new SniSslEngineFactory(sslContext))
          .addContactEndPoints(builtEndpoints)
          .build();
      } else if (args[0].equals("mtls-sni-ingress")) {
        DriverConfigLoader configLoader = DriverConfigLoader.fromClasspath("mtls-sni-ingress.conf");
        DriverExecutionProfile config = configLoader.getInitialConfig().getDefaultProfile();

        String ingressAddress = config.getString(KubernetesOption.INGRESS_ADDRESS);
        int ingressPort = config.getInt(KubernetesOption.INGRESS_PORT);
        InetSocketAddress cloudProxyAddress = new InetSocketAddress(ingressAddress, ingressPort);

        Set<EndPoint> builtEndpoints = new HashSet<>();
        List<String> configEndpoints = config.getStringList(KubernetesOption.ENDPOINTS);
        for (String configEndpoint : configEndpoints) {
          builtEndpoints.add(new SniEndPoint(cloudProxyAddress, configEndpoint));
        }

        SSLContext sslContext = buildContext(config);

        session = CqlSession.builder()
          .withConfigLoader(configLoader)
          .withCloudProxyAddress(cloudProxyAddress)
          .withSslEngineFactory(new SniSslEngineFactory(sslContext))
          .addContactEndPoints(builtEndpoints)
          .build();
      }

      if (session != null) {
        SampleApp app = new SampleApp();
        app.run(session);
      } else {
        System.err.println("Invalid connection type. Possible options:");
        printConnectionTypes();
      }
    }
  }

  private static SSLContext buildContext(DriverExecutionProfile config) throws Exception {
      if (config.isDefined(DefaultDriverOption.SSL_KEYSTORE_PATH)
          || config.isDefined(DefaultDriverOption.SSL_TRUSTSTORE_PATH)) {
        SSLContext context = SSLContext.getInstance("SSL");
  
        // initialize truststore if configured.
        TrustManagerFactory tmf = null;
        if (config.isDefined(DefaultDriverOption.SSL_TRUSTSTORE_PATH)) {
          try (InputStream tsf =
              Files.newInputStream(
                  Paths.get(config.getString(DefaultDriverOption.SSL_TRUSTSTORE_PATH)))) {
            KeyStore ts = KeyStore.getInstance("JKS");
            char[] password =
                config.isDefined(DefaultDriverOption.SSL_TRUSTSTORE_PASSWORD)
                    ? config.getString(DefaultDriverOption.SSL_TRUSTSTORE_PASSWORD).toCharArray()
                    : null;
            ts.load(tsf, password);
            tmf = TrustManagerFactory.getInstance(TrustManagerFactory.getDefaultAlgorithm());
            tmf.init(ts);
          }
        }
  
        // initialize keystore if configured.
        KeyManagerFactory kmf = null;
        if (config.isDefined(DefaultDriverOption.SSL_KEYSTORE_PATH)) {
          try (InputStream ksf =
              Files.newInputStream(
                  Paths.get(config.getString(DefaultDriverOption.SSL_KEYSTORE_PATH)))) {
            KeyStore ks = KeyStore.getInstance("JKS");
            char[] password =
                config.isDefined(DefaultDriverOption.SSL_KEYSTORE_PASSWORD)
                    ? config.getString(DefaultDriverOption.SSL_KEYSTORE_PASSWORD).toCharArray()
                    : null;
            ks.load(ksf, password);
            kmf = KeyManagerFactory.getInstance(KeyManagerFactory.getDefaultAlgorithm());
            kmf.init(ks, password);
          }
        }
  
        context.init(
            kmf != null ? kmf.getKeyManagers() : null,
            tmf != null ? tmf.getTrustManagers() : null,
            new SecureRandom());
        return context;
      } else {
        // if both keystore and truststore aren't configured, use default SSLContext.
        return SSLContext.getDefault();
      }
  }

  private static void printConnectionTypes() {
    System.err.println("  direct");
    System.err.println("  ingress");
    System.err.println("  sni-ingress");
    System.err.println("  mtls-sni-ingress");
  }

  public void run(CqlSession session) throws Exception {
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
}
