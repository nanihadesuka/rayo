package rayo;

import static rayo.Screen.*;
import static rayo.Shell.*;
import static rayo.Timing.*;
import static rayo.Ui.*;

import android.graphics.Rect;
import android.view.accessibility.AccessibilityNodeInfo;
import android.os.SystemClock;
import java.util.regex.Pattern;

/** Waits for the screen, reacting to its changes. */
final class Waiting {
  private Waiting() {}

  static final int POLL_MS = 50;
  static final int STILL_MS = 34;
  static final int RECENT_MS = 1500;
  static final int SYSTEM_MS = 500;
  static final int FRESH_MS = 100;

  static long seen() {
    synchronized (changed) {
      return changes;
    }
  }

  /** Returns at the next screen change, or after a short while in case one goes unreported. */
  static void awaitChange(long since, long deadline) throws Exception {
    long began = System.nanoTime();
    long until = Math.min(deadline, SystemClock.uptimeMillis() + POLL_MS);
    synchronized (changed) {
      for (long left; changes == since && (left = until - SystemClock.uptimeMillis()) > 0; ) changed.wait(left);
    }
    spend("wait", began);
  }

  /** Waits until nothing has changed for a while. */
  static void settle(long quiet, long limit) throws Exception {
    long start = SystemClock.uptimeMillis();
    synchronized (changed) {
      while (true) {
        long now = SystemClock.uptimeMillis();
        long left = Math.max(start, lastChange) + quiet - now;
        if (left <= 0 || now - start >= limit) return;
        changed.wait(left);
      }
    }
  }

  // What the last search found: a tap right after it needs no second look to know it is still.
  static String lastSought;
  static Rect lastBounds;
  static long lastFound;

  static Entry await(Selector s, long deadline) throws Exception {
    long systemAt = SystemClock.uptimeMillis() + SYSTEM_MS;
    long freshAt = 0;
    while (true) {
      long since = seen();
      long now = SystemClock.uptimeMillis();
      Entry e = s.simple() ? confirmed(s) : null;
      if (e == null && (now >= freshAt || now >= deadline)) {
        boolean system = now >= systemAt || now >= deadline;
        e = s.first(entries(system)); // also notifications and the status bar, now and then
        now = SystemClock.uptimeMillis();
        if (system) systemAt = now + SYSTEM_MS;
        freshAt = now + FRESH_MS;
      }
      if (e != null) {
        lastSought = s.toString();
        lastBounds = e.bounds;
        lastFound = now;
        return e;
      }
      if (now >= deadline) throw new Failure("not on screen: " + s);
      awaitChange(since, deadline);
      if (seen() != since) freshAt = 0;
    }
  }

  /**
   * A read of everything takes several trips to the app, and each waits its turn while the app is
   * busy. Finding the element in what is remembered takes none, and asking the app if it is still
   * so takes one.
   */
  static Entry confirmed(Selector s) throws Exception {
    Entry e;
    try {
      e = s.first(remembered());
    } catch (RuntimeException x) {
      return null; // only a shortcut: the fresh read decides (#38)
    }
    if (e == null) return null;
    Rect b = again(e, s);
    return b == null ? null : new Entry(e.node, b, -1);
  }

  static void awaitGone(Selector s, long deadline) throws Exception {
    while (true) {
      long since = seen();
      if (s.first(entries()) == null) return;
      if (SystemClock.uptimeMillis() >= deadline) throw new Failure("still on screen: " + s);
      awaitChange(since, deadline);
    }
  }

  /** Where the element is, once it stops moving and is enabled: earlier taps can be lost. */
  static Rect awaitStill(Selector s, long deadline) throws Exception {
    long age = SystemClock.uptimeMillis() - lastFound;
    Rect last = s.toString().equals(lastSought) && age >= STILL_MS && age < RECENT_MS ? lastBounds : null;
    while (true) {
      Entry e = await(s, deadline);
      boolean ready = e.node.isEnabled() || s.json.has("enabled");
      if (ready && e.bounds.equals(last)) return e.bounds;
      long began = System.nanoTime();
      SystemClock.sleep(STILL_MS);
      spend("still", began);
      last = ready ? again(e, s) : null;
      if (e.bounds.equals(last)) return last;
      if (SystemClock.uptimeMillis() >= deadline) {
        throw new Failure((ready ? "never stopped moving: " : "still disabled: ") + s);
      }
    }
  }

  /** Where the element is now, reading only it and not the whole screen; null if it isn't. */
  static Rect again(Entry e, Selector s) {
    long began = System.nanoTime();
    Rect b = null;
    try {
      if (refresh(e.node) && e.node.isVisibleToUser() && s.self(e.node)) {
        b = new Rect();
        e.node.getBoundsInScreen(b);
        if (!b.intersect(screen())) b = null;
      }
    } catch (RuntimeException x) {
      b = null; // its window was just replaced (#38): not there
    }
    spend("check", began);
    return b;
  }

  /** The list the element scrolls in, as much of it as is on screen; the screen if none. */
  static Rect list(Entry e) {
    Rect c = new Rect(screen());
    try {
      for (AccessibilityNodeInfo p = e.node.getParent(); p != null; p = p.getParent()) {
        if (p.isScrollable()) {
          Rect r = new Rect();
          p.getBoundsInScreen(r);
          c.intersect(r);
          break;
        }
      }
    } catch (RuntimeException x) {
      // its window was just replaced (#38): the screen will do
    }
    return c;
  }

  /**
   * Not cut off by the list it scrolls in, along the way it scrolls: neither end of it is on an
   * end of the list (#39). Unknown is not whole: one more scroll looks again.
   */
  static boolean whole(Entry e, Rect c, boolean across) {
    Rect b = e.bounds;
    return across ? b.left > c.left && b.right < c.right : b.top > c.top && b.bottom < c.bottom;
  }

  /** How far its middle is past the middle of the list, as a share of the list: negative before it. */
  static double past(Entry e, Rect c, boolean across) {
    Rect b = e.bounds;
    return across ? (b.exactCenterX() - c.exactCenterX()) / c.width() : (b.exactCenterY() - c.exactCenterY()) / c.height();
  }

  /** Asks the system, not the app: one that is busy starting answers late. */
  static void awaitFront(String app, long deadline) throws Exception {
    Pattern resumed = Pattern.compile("Hist[^\n]*" + Pattern.quote(" " + app + "/") + "[^\n]*\n[^H]*state=RESUMED");
    for (; !resumed.matcher(states(app)).find(); SystemClock.sleep(20)) {
      if (SystemClock.uptimeMillis() >= deadline) throw new Failure("not in front: " + app);
    }
  }

  /** One line per activity, followed by its state. */
  static String states(String app) throws Exception {
    return shell("dumpsys activity activities " + app + " | grep -oE 'Hist +#.*|state=[A-Z]+'");
  }

  /** After Home: waits until the app has really stopped, or reopening it could go unnoticed. */
  static void awaitStopped(String app) throws Exception {
    Pattern busy = Pattern.compile("Hist[^\n]*" + Pattern.quote(" " + app + "/") + "[^\n]*\n[^H]*state=(RESUMED|PAUSING|PAUSED|STOPPING)");
    for (long deadline = SystemClock.uptimeMillis() + 3000; SystemClock.uptimeMillis() < deadline; SystemClock.sleep(20)) {
      String states = states(app);
      if (!busy.matcher(states).find()) return;
    }
  }
}
