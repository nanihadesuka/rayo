package rayo;

import static rayo.Animations.*;
import static rayo.Input.*;
import static rayo.Location.*;
import static rayo.Screen.*;
import static rayo.Shell.*;
import static rayo.Timing.*;
import static rayo.Ui.*;
import static rayo.Waiting.*;

import android.graphics.Rect;
import android.os.SystemClock;
import android.view.KeyEvent;
import java.io.BufferedReader;
import java.io.InputStreamReader;
import java.io.PrintStream;
import java.nio.charset.StandardCharsets;
import java.util.concurrent.LinkedBlockingQueue;
import org.json.JSONObject;

/**
 * Runs on the device as the shell user (app_process, nothing installed). Reads one JSON request
 * per line on stdin and answers with one "@@"-prefixed JSON line on stdout. Waits happen here,
 * next to the screen. When stdin closes, however the host died, it undoes what it changed.
 */
public final class Agent {
  static PrintStream out;

  public static void main(String[] args) {
    out = new PrintStream(System.out, true);
    owner = Thread.currentThread();
    int code = 0;
    try {
      connect();
      Runtime.getRuntime().addShutdownHook(new Thread(Agent::restore));
      reply(new JSONObject().put("ready", true));
      LinkedBlockingQueue<String> lines = new LinkedBlockingQueue<>();
      Thread reader = new Thread(() -> read(lines), "rayo-reader");
      reader.setDaemon(true);
      reader.start();
      while (true) {
        String line = lines.take();
        JSONObject answer;
        spent.clear();
        long began = System.nanoTime();
        try {
          answer = handle(new JSONObject(line));
        } catch (Failure e) {
          answer = new JSONObject().put("error", e.getMessage());
        } catch (Throwable e) {
          android.util.Log.e("rayo", "agent", e);
          answer = new JSONObject().put("error", "agent: " + e + where(e));
        }
        spend("device", began);
        reply(answer.put("spent", new JSONObject(spent)));
        if (ui == null) break; // said bye
      }
    } catch (Throwable e) {
      e.printStackTrace(out);
      code = 1;
    }
    System.exit(code); // runs the shutdown hook
  }

  /** On its own, so that a lost host is noticed also during a long wait. */
  private static void read(LinkedBlockingQueue<String> lines) {
    try {
      BufferedReader in = new BufferedReader(new InputStreamReader(System.in, StandardCharsets.UTF_8));
      for (String line; (line = in.readLine()) != null; ) lines.add(line);
    } catch (Throwable e) {
      // The same as the end.
    }
    System.exit(0); // runs the shutdown hook
  }

  static void reply(JSONObject answer) {
    out.println("@@" + answer);
  }

  static synchronized void restore() {
    attempt(Animations::awaitQuiet);
    attempt(Ui::disconnect); // or Android notices that this process is gone
    attempt(Location::stopLocation);
    attempt(Animations::restoreAnimations);
  }

  private interface Attempt {
    void run() throws Exception;
  }

  /** Restores as far as it goes: one that fails must not keep the others from it. */
  private static void attempt(Attempt a) {
    try {
      a.run();
    } catch (Throwable e) {
      e.printStackTrace(); // the host shows it if the device is lost
    }
  }

  static JSONObject handle(JSONObject r) throws Exception {
    JSONObject answer = new JSONObject();
    long deadline = SystemClock.uptimeMillis() + r.optLong("timeout");
    String op = r.getString("op");
    if (op.equals("tapAt") || op.startsWith("swipe") || op.equals("key")) tapped = null; // focus may have moved
    switch (op) {
      case "wait": {
        Entry e = await(new Selector(r.getJSONObject("sel")), deadline);
        answer.put("text", e.label());
        if (r.has("across")) {
          Rect list = list(e);
          answer.put("whole", whole(e, list, r.getBoolean("across")));
          answer.put("past", past(e, list, r.getBoolean("across")));
        }
        break;
      }
      case "gone":
        awaitGone(new Selector(r.getJSONObject("sel")), deadline);
        break;
      case "tap": {
        Rect b = awaitStill(new Selector(r.getJSONObject("sel")), deadline);
        float x = b.left + b.width() * (float) r.optDouble("fx", 0.5);
        float y = b.top + b.height() * (float) r.optDouble("fy", 0.5);
        presses(x, y, r);
        tapped = new android.graphics.Point((int) x, (int) y);
        break;
      }
      case "tapAt":
        presses(at(r, "x", screen().width()), at(r, "y", screen().height()), r);
        break;
      case "swipe": {
        Rect s = screen();
        swipe(at(r, "x", s.width()), at(r, "y", s.height()), at(r, "x2", s.width()), at(r, "y2", s.height()), r.optLong("ms", 250), r.optBoolean("hold"));
        break;
      }
      case "swipeOn": {
        Rect b = awaitStill(new Selector(r.getJSONObject("sel")), deadline);
        Rect s = screen();
        float dx = (float) r.getDouble("dx") * s.width();
        float dy = (float) r.getDouble("dy") * s.height();
        float x = b.exactCenterX();
        float y = b.exactCenterY();
        swipe(x, y, clamp(x + dx, s.width()), clamp(y + dy, s.height()), r.optLong("ms", 250), r.optBoolean("hold"));
        break;
      }
      case "set":
        set(new Selector(r.getJSONObject("sel")), r.getString("value"), deadline);
        break;
      case "input":
        input(r.getString("value"), deadline);
        break;
      case "erase":
        erase(r.getInt("count"), deadline);
        break;
      case "key":
        if (r.getInt("code") == KeyEvent.KEYCODE_HOME) {
          String app = front();
          key(KeyEvent.KEYCODE_HOME);
          awaitStopped(app);
        } else {
          key(r.getInt("code"));
          settle(34, 120);
        }
        break;
      case "hideKeyboard":
        hideKeyboard(deadline);
        break;
      case "settle":
        settle(r.optLong("quiet", 200), r.optLong("timeout"));
        break;
      case "front":
        awaitFront(r.getString("app"), deadline);
        break;
      case "location":
        startLocation(r.getString("place"));
        break;
      case "resetLocation":
        stopLocation();
        break;
      case "noAnimations":
        noAnimations();
        break;
      case "shell":
        if (r.optBoolean("quiet")) awaitQuiet();
        answer.put("out", shell(r.getString("command")));
        break;
      case "bye":
        restore();
        break;
      case "tree":
        answer.put("out", tree());
        break;
      default:
        throw new Failure("unknown request " + r.getString("op"));
    }
    return answer;
  }

  static float at(JSONObject r, String key, int size) throws Exception {
    double v = r.getDouble(key);
    return (float) (r.optBoolean("percent") ? clamp((float) (v * size / 100), size) : v);
  }

  static float clamp(float v, int size) {
    return Math.max(1, Math.min(v, size - 2));
  }

  /** The first frames, so a failure inside Android can be traced from the report alone. */
  static String where(Throwable e) {
    StringBuilder at = new StringBuilder();
    StackTraceElement[] frames = e.getStackTrace();
    for (int i = 0; i < Math.min(frames.length, 8); i++) at.append("\n  at ").append(frames[i]);
    return at.toString();
  }
}
