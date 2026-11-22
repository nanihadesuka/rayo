package rayo;

import static rayo.Timing.*;

import java.io.ByteArrayOutputStream;
import java.io.InputStream;

/** Commands, as the shell user. */
final class Shell {
  private Shell() {}

  static String shell(String command) throws Exception {
    long began = System.nanoTime();
    Process p = new ProcessBuilder("sh", "-c", command).redirectErrorStream(true).start();
    p.getOutputStream().close();
    ByteArrayOutputStream all = new ByteArrayOutputStream();
    byte[] buffer = new byte[8192];
    try (InputStream in = p.getInputStream()) {
      for (int n; (n = in.read(buffer)) > 0; ) all.write(buffer, 0, n);
    }
    p.waitFor();
    spend("shell", began);
    return all.toString("UTF-8");
  }
}
