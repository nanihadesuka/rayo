package rayo;

import android.accessibilityservice.AccessibilityServiceInfo;
import android.app.UiAutomation;
import android.graphics.Rect;
import android.os.Build;
import android.os.HandlerThread;
import android.os.Looper;
import android.os.SystemClock;
import android.view.accessibility.AccessibilityEvent;
import android.view.accessibility.AccessibilityNodeInfo;
import android.view.accessibility.AccessibilityWindowInfo;
import java.lang.reflect.Constructor;
import java.lang.reflect.Method;
import java.util.ArrayList;
import java.util.List;

/** The connection to Android's accessibility: windows and their changes. */
final class Ui {
  private Ui() {}

  static volatile UiAutomation ui;
  static final Object changed = new Object();
  static long changes;
  static long lastChange;

  static void connect() throws Exception {
    if (Looper.getMainLooper() == null) Looper.prepareMainLooper();
    HandlerThread thread = new HandlerThread("rayo");
    thread.setDaemon(true);
    thread.start();

    // Android takes a moment to let go of a previous run.
    for (long deadline = SystemClock.uptimeMillis() + 5000; ; SystemClock.sleep(50)) {
      try {
        ui = open(thread.getLooper());
        break;
      } catch (java.lang.reflect.InvocationTargetException e) {
        if (SystemClock.uptimeMillis() < deadline && String.valueOf(e.getCause()).contains("already registered")) continue;
        throw e;
      }
    }

    AccessibilityServiceInfo info = ui.getServiceInfo();
    info.flags |= AccessibilityServiceInfo.FLAG_RETRIEVE_INTERACTIVE_WINDOWS
        | AccessibilityServiceInfo.FLAG_REPORT_VIEW_IDS
        | AccessibilityServiceInfo.FLAG_INCLUDE_NOT_IMPORTANT_VIEWS;
    info.notificationTimeout = 0;
    // Only those that mean the screen changed: each event costs the app.
    info.eventTypes = AccessibilityEvent.TYPE_WINDOW_CONTENT_CHANGED | AccessibilityEvent.TYPE_WINDOW_STATE_CHANGED
        | AccessibilityEvent.TYPE_WINDOWS_CHANGED | AccessibilityEvent.TYPE_VIEW_TEXT_CHANGED
        | AccessibilityEvent.TYPE_VIEW_FOCUSED | AccessibilityEvent.TYPE_VIEW_SCROLLED;
    ui.setServiceInfo(info);
    serviceInfo = info;
    ui.setOnAccessibilityEventListener(event -> {
      synchronized (changed) {
        changes++;
        lastChange = SystemClock.uptimeMillis();
        changed.notifyAll();
      }
    });
  }

  static UiAutomation open(Looper looper) throws Exception {
    Constructor<?> newConnection = Class.forName("android.app.UiAutomationConnection").getDeclaredConstructor();
    newConnection.setAccessible(true);
    Object connection = newConnection.newInstance();

    // (Looper, IUiAutomationConnection), with a leading display id on some versions.
    for (Constructor<?> c : UiAutomation.class.getDeclaredConstructors()) {
      Class<?>[] types = c.getParameterTypes();
      Object[] values = new Object[types.length];
      boolean ok = types.length >= 2;
      for (int i = 0; i < types.length; i++) {
        if (types[i] == Looper.class) values[i] = looper;
        else if (types[i] == int.class) values[i] = 0;
        else if (types[i].isInstance(connection)) values[i] = connection;
        else ok = false;
      }
      if (!ok) continue;
      c.setAccessible(true);
      UiAutomation made = (UiAutomation) c.newInstance(values);
      Method connect = UiAutomation.class.getDeclaredMethod("connect", int.class);
      connect.setAccessible(true);
      connect.invoke(made, UiAutomation.FLAG_DONT_SUPPRESS_ACCESSIBILITY_SERVICES);
      return made;
    }
    throw new IllegalStateException("unsupported Android version: no UiAutomation constructor");
  }

  // The app answers between frames, so every trip to it can be slow: bring as much as fits in one.
  static final int PREFETCH = Build.VERSION.SDK_INT >= 33
      ? AccessibilityNodeInfo.FLAG_PREFETCH_DESCENDANTS_BREADTH_FIRST | AccessibilityNodeInfo.FLAG_PREFETCH_UNINTERRUPTIBLE
      : 0;

  /** The topmost window first: the apps' windows, or the system's, such as the status bar. */
  static List<AccessibilityNodeInfo> roots(boolean apps) {
    List<AccessibilityNodeInfo> roots = new ArrayList<>();
    for (AccessibilityWindowInfo w : windows()) {
      if (w.getType() == AccessibilityWindowInfo.TYPE_INPUT_METHOD) continue;
      if ((w.getType() == AccessibilityWindowInfo.TYPE_APPLICATION) != apps) continue;
      AccessibilityNodeInfo root = root(w, PREFETCH);
      if (root != null) roots.add(root);
    }
    return roots;
  }

  // Android's accessibility answers from other processes, any of which may be starting, dying or
  // being replaced, and its own code has bugs there (#38). Every call into it goes through these:
  // a throw, a null list or a null item is "nothing there", and the waits look again.

  /** The windows, the topmost first; empty when Android can't say. */
  static List<AccessibilityWindowInfo> windows() {
    List<AccessibilityWindowInfo> all = new ArrayList<>();
    try {
      List<AccessibilityWindowInfo> listed = ui.getWindows();
      if (listed != null) {
        for (AccessibilityWindowInfo w : listed) if (w != null) all.add(w);
      }
    } catch (RuntimeException e) {
      // nothing to read yet
    }
    return all;
  }

  static int childCount(AccessibilityNodeInfo n) {
    try {
      return n.getChildCount();
    } catch (RuntimeException e) {
      return 0;
    }
  }

  /** The node with input focus, or null. */
  static AccessibilityNodeInfo inputFocus(AccessibilityNodeInfo in) {
    try {
      return in == null ? ui.findFocus(AccessibilityNodeInfo.FOCUS_INPUT) : in.findFocus(AccessibilityNodeInfo.FOCUS_INPUT);
    } catch (RuntimeException e) {
      return null;
    }
  }

  static AccessibilityNodeInfo activeRoot() {
    try {
      return ui.getRootInActiveWindow();
    } catch (RuntimeException e) {
      return null;
    }
  }

  /** False also when its window went before it could act. */
  static boolean act(AccessibilityNodeInfo n, int action, android.os.Bundle args) {
    try {
      return n.performAction(action, args);
    } catch (RuntimeException e) {
      return false;
    }
  }

  /** Read anew; false when it is gone, however Android says so. */
  static boolean refresh(AccessibilityNodeInfo n) {
    try {
      return n.refresh();
    } catch (RuntimeException e) {
      return false;
    }
  }

  /**
   * Android 13 can throw, instead of answering null, for a node whose window or process has just
   * been replaced (#38): to a reader, that node is simply not there.
   */
  static AccessibilityNodeInfo child(AccessibilityNodeInfo n, int i, int prefetch) {
    try {
      return prefetch == 0 ? n.getChild(i) : n.getChild(i, prefetch);
    } catch (RuntimeException e) {
      return null;
    }
  }

  static AccessibilityNodeInfo root(AccessibilityWindowInfo w, int prefetch) {
    try {
      return prefetch == 0 ? w.getRoot() : w.getRoot(prefetch);
    } catch (RuntimeException e) {
      return null;
    }
  }

  static AccessibilityServiceInfo serviceInfo;
  static Method clear;
  static boolean noClear;

  /**
   * Otherwise elements can be read as they were a moment ago, or, from an app that doesn't say when
   * it changes (Compose that thinks accessibility is off), as they were when first read.
   */
  static void clearCache() {
    if (!noClear) {
      try {
        if (clear == null) clear = UiAutomation.class.getMethod("clearCache"); // Android 14+
        clear.invoke(ui);
        return;
      } catch (Throwable e) {
        noClear = true;
      }
    }
    ui.setServiceInfo(serviceInfo); // before 14: the same settings again, which empties the cache
  }

  static Rect cachedScreen;
  static long screenAt;

  static Rect screen() {
    long now = SystemClock.uptimeMillis();
    if (cachedScreen == null || cachedScreen.isEmpty() || now - screenAt > 1000) {
      Rect all = new Rect();
      for (AccessibilityWindowInfo w : windows()) {
        Rect b = new Rect();
        w.getBoundsInScreen(b);
        all.union(b);
      }
      cachedScreen = all;
      screenAt = now;
    }
    return cachedScreen;
  }

  static String front() {
    clearCache();
    AccessibilityNodeInfo root = activeRoot();
    return root == null ? "" : String.valueOf(root.getPackageName());
  }

  static void disconnect() throws Exception {
    UiAutomation old = ui;
    ui = null;
    if (old == null) return;
    Method disconnect = UiAutomation.class.getDeclaredMethod("disconnect");
    disconnect.setAccessible(true);
    disconnect.invoke(old);
  }
}
