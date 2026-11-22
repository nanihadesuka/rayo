package rayo;

import static rayo.Shell.*;

/** A pretend location. */
final class Location {
  private Location() {}

  static final int GPS_MS = 1000;
  static final String[] PROVIDERS = {"gps", "network", "fused"};

  static Thread gps;
  static volatile String place;

  /** Stays at a place, sending fixes like a real GPS. */
  static void startLocation(String at) throws Exception {
    place = at;
    StringBuilder fix = new StringBuilder();
    for (String p : PROVIDERS) {
      fix.append("cmd location providers set-test-provider-location ").append(p).append(" --location %s --accuracy 3 & ");
    }
    String command = fix + "wait";
    if (gps != null) {
      shell(command.replace("%s", at));
      return;
    }
    StringBuilder setup = new StringBuilder("appops set com.android.shell android:mock_location allow;");
    for (String p : PROVIDERS) {
      setup.append("{ cmd location providers add-test-provider ").append(p).append("; ");
      setup.append("cmd location providers set-test-provider-enabled ").append(p).append(" true; } & ");
    }
    shell(setup + "wait; " + command.replace("%s", at));
    gps = new Thread(() -> {
      try {
        while (true) {
          Thread.sleep(GPS_MS);
          shell(command.replace("%s", place));
        }
      } catch (Exception e) {
        // stopLocation() ends it.
      }
    });
    gps.setName("rayo-gps");
    gps.setDaemon(true);
    gps.start();
  }

  static void stopLocation() throws Exception {
    if (gps == null) return;
    gps.interrupt();
    gps.join(2000);
    gps = null;
    StringBuilder undo = new StringBuilder();
    for (String p : PROVIDERS) undo.append("cmd location providers remove-test-provider ").append(p).append(';');
    shell(undo + "appops set com.android.shell android:mock_location default");
  }
}
