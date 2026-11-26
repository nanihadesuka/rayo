package rayo;

import static rayo.Timing.*;
import static rayo.Ui.*;
import static rayo.Waiting.*;

import android.os.Bundle;
import android.os.SystemClock;
import android.view.InputDevice;
import android.view.KeyCharacterMap;
import android.view.KeyEvent;
import android.view.MotionEvent;
import android.view.accessibility.AccessibilityNodeInfo;
import android.view.accessibility.AccessibilityWindowInfo;
import org.json.JSONObject;

/** Touches, keys and text. */
final class Input {
  private Input() {}

  static void presses(float x, float y, JSONObject r) throws Exception {
    int times = r.optInt("repeat", 1);
    for (int i = 0; i < times; i++) {
      if (i > 0) SystemClock.sleep(r.optLong("delay", 100));
      long down = SystemClock.uptimeMillis();
      touch(down, MotionEvent.ACTION_DOWN, x, y);
      if (r.optLong("hold") > 0) SystemClock.sleep(r.optLong("hold"));
      touch(down, MotionEvent.ACTION_UP, x, y);
    }
  }

  static void swipe(float x, float y, float x2, float y2, long ms) throws Exception {
    swipe(x, y, x2, y2, ms, false);
  }

  /** With hold, the finger stops before it lifts: the list moves just as far, and does not fling past (#39). */
  static void swipe(float x, float y, float x2, float y2, long ms, boolean hold) throws Exception {
    long down = SystemClock.uptimeMillis();
    touch(down, MotionEvent.ACTION_DOWN, x, y);
    for (long t; (t = SystemClock.uptimeMillis() - down) < ms; SystemClock.sleep(8)) {
      float f = (float) t / ms;
      touch(down, MotionEvent.ACTION_MOVE, x + (x2 - x) * f, y + (y2 - y) * f);
    }
    touch(down, MotionEvent.ACTION_MOVE, x2, y2);
    for (long still = SystemClock.uptimeMillis(); hold && SystemClock.uptimeMillis() - still < 150; SystemClock.sleep(16)) {
      touch(down, MotionEvent.ACTION_MOVE, x2, y2);
    }
    touch(down, MotionEvent.ACTION_UP, x2, y2);
    // Until a fling ends: Compose tells of a change at most every 100 ms, so less quiet is not the end (#39).
    settle(160, 3000);
  }

  static void touch(long down, int action, float x, float y) throws Exception {
    MotionEvent e = MotionEvent.obtain(down, SystemClock.uptimeMillis(), action, x, y, 0);
    e.setSource(InputDevice.SOURCE_TOUCHSCREEN);
    long began = System.nanoTime();
    boolean ok = ui.injectInputEvent(e, action == MotionEvent.ACTION_UP);
    spend("inject", began);
    e.recycle();
    if (!ok) throw new Failure("the device refused a touch at " + (int) x + "," + (int) y);
  }

  static void key(int code) throws Exception {
    long now = SystemClock.uptimeMillis();
    for (int action : new int[] {KeyEvent.ACTION_DOWN, KeyEvent.ACTION_UP}) {
      KeyEvent e = new KeyEvent(now, SystemClock.uptimeMillis(), action, code, 0, 0,
          KeyCharacterMap.VIRTUAL_KEYBOARD, 0, 0, InputDevice.SOURCE_KEYBOARD);
      long began = System.nanoTime();
      boolean ok = ui.injectInputEvent(e, true);
      spend("inject", began);
      if (!ok) throw new Failure("the device refused key " + code);
    }
  }

  static boolean put(AccessibilityNodeInfo field, String value) {
    Bundle args = new Bundle();
    args.putCharSequence(AccessibilityNodeInfo.ACTION_ARGUMENT_SET_TEXT_CHARSEQUENCE, value);
    return act(field, AccessibilityNodeInfo.ACTION_SET_TEXT, args);
  }

  static boolean editable(AccessibilityNodeInfo n) {
    java.util.List<AccessibilityNodeInfo.AccessibilityAction> actions = n.getActionList();
    return n.isEnabled() && actions != null && actions.contains(AccessibilityNodeInfo.AccessibilityAction.ACTION_SET_TEXT);
  }

  /** Replaces the text of a field, without focus or keyboard. */
  static void set(Selector s, String value, long deadline) throws Exception {
    while (true) {
      long since = seen();
      AccessibilityNodeInfo field = inside(await(s, deadline).node);
      if (field != null && put(field, value)) return;
      if (SystemClock.uptimeMillis() >= deadline) throw new Failure("not a text field: " + s);
      awaitChange(since, deadline);
    }
  }

  static AccessibilityNodeInfo inside(AccessibilityNodeInfo n) {
    if (editable(n)) return n;
    for (int i = 0, count = childCount(n); i < count; i++) {
      AccessibilityNodeInfo child = child(n, i, 0);
      AccessibilityNodeInfo found = child == null ? null : inside(child);
      if (found != null) return found;
    }
    return null;
  }

  /**
   * Where the last tapOn touched: a field there is the one meant, even when accessibility still reports the field
   * focused before (Compose may, #16). Another touch or key forgets it.
   */
  static android.graphics.Point tapped;

  static AccessibilityNodeInfo focusedField(long deadline) throws Exception {
    while (true) {
      long since = seen();
      clearCache();
      String[] why = {"no input focus"};
      AccessibilityNodeInfo n;
      try {
        n = findField(why);
      } catch (RuntimeException e) { // a window being replaced (#38): look again
        n = null;
        why[0] = "the screen was changing: " + e;
      }
      if (n != null) return n;
      if (SystemClock.uptimeMillis() >= deadline) throw new Failure("no text field is focused: " + why[0]);
      awaitChange(since, deadline);
    }
  }

  /** The field: where the last tap was, then where accessibility says, then in every window (a sheet or dialog is its own). */
  static AccessibilityNodeInfo findField(String[] why) {
    java.util.List<AccessibilityWindowInfo> windows = windows();
    AccessibilityNodeInfo f;
    if (tapped != null) {
      for (AccessibilityWindowInfo w : windows) { // the top one first
        AccessibilityNodeInfo root = root(w, 0);
        if (root != null && (f = at(root, tapped.x, tapped.y)) != null) {
          // Keys typed go where the input focus is: move it here if it isn't.
          if (!f.isFocused()) act(f, AccessibilityNodeInfo.ACTION_FOCUS, null);
          return f;
        }
      }
    }
    AccessibilityNodeInfo n = inputFocus(null);
    f = n == null ? null : inside(n);
    if (f != null) return f;
    if (n != null) why[0] = "focused node " + n.getClassName() + " has no SET_TEXT";
    for (AccessibilityWindowInfo w : windows) {
      AccessibilityNodeInfo root = root(w, 0);
      if (root == null) continue;
      AccessibilityNodeInfo m = inputFocus(root);
      if (m != null && (f = inside(m)) != null) return f;
      if ((f = focusedIn(root)) != null) return f;
    }
    return null;
  }

  static AccessibilityNodeInfo focusedIn(AccessibilityNodeInfo n) {
    if (n.isFocused() && editable(n)) return n;
    for (int i = 0, count = childCount(n); i < count; i++) {
      AccessibilityNodeInfo child = child(n, i, 0);
      AccessibilityNodeInfo found = child == null ? null : focusedIn(child);
      if (found != null) return found;
    }
    return null;
  }

  static AccessibilityNodeInfo at(AccessibilityNodeInfo n, int x, int y) {
    android.graphics.Rect b = new android.graphics.Rect();
    n.getBoundsInScreen(b);
    if (!b.contains(x, y)) return null;
    for (int i = 0, count = childCount(n); i < count; i++) {
      AccessibilityNodeInfo child = child(n, i, 0);
      AccessibilityNodeInfo found = child == null ? null : at(child, x, y);
      if (found != null) return found;
    }
    return editable(n) ? n : null;
  }

  static String textOf(AccessibilityNodeInfo field) {
    return field.getText() == null || field.isShowingHintText() ? "" : field.getText().toString();
  }

  /** Adds to the focused field. */
  static void input(String value, long deadline) throws Exception {
    AccessibilityNodeInfo field = focusedField(deadline);
    String old = textOf(field);
    if (old.isEmpty()) {
      if (!put(field, value)) throw new Failure("the field doesn't accept text");
    } else {
      // Passwords read as dots, so they can't be rewritten: type as a keyboard would.
      KeyEvent[] events = KeyCharacterMap.load(KeyCharacterMap.VIRTUAL_KEYBOARD).getEvents(value.toCharArray());
      if (events != null) {
        for (KeyEvent e : events) ui.injectInputEvent(e, true);
      } else if (field.isPassword() || !put(field, old + value)) {
        throw new Failure("can't add this text to the field");
      }
    }
  }

  static void erase(int count, long deadline) throws Exception {
    AccessibilityNodeInfo field = focusedField(deadline);
    String old = textOf(field);
    if (!field.isPassword() || old.length() <= count) {
      if (put(field, old.substring(0, Math.max(0, old.length() - count)))) return;
    }
    for (int i = 0; i < count; i++) key(KeyEvent.KEYCODE_DEL);
  }

  /**
   * An input method window as tall as a keyboard: with a hardware keyboard, the IME may keep a small toolbar
   * window, and Back would then close what is under it instead.
   */
  static boolean keyboardShown() {
    android.graphics.Rect b = new android.graphics.Rect();
    int tall = Math.max(screen().height() / 8, 1);
    for (AccessibilityWindowInfo w : windows()) {
      if (w.getType() != AccessibilityWindowInfo.TYPE_INPUT_METHOD) continue;
      w.getBoundsInScreen(b);
      if (b.height() >= tall) return true;
    }
    return false;
  }

  /** What the input method service says; the windows accessibility lists can lag behind it. */
  static boolean imeShown() {
    try {
      String state = Shell.shell("dumpsys input_method");
      if (state.contains("mInputShown=true") || state.contains("mIsInputViewShown=true")) return true;
      return !state.contains("mInputShown=");
    } catch (Exception e) {
      return true; // can't tell, so the windows decide
    }
  }

  static boolean keyboardOpen() { return keyboardShown() && imeShown(); }

  static void hideKeyboard(long deadline) throws Exception {
    settle(60, 300); // it may be opening
    if (!keyboardOpen()) return; // nothing to do
    key(KeyEvent.KEYCODE_BACK); // one Back only: a second one could close the window under the keyboard
    while (keyboardOpen()) {
      long now = SystemClock.uptimeMillis();
      if (now >= deadline) throw new Failure("the keyboard didn't close after one Back; to send another, use back or retry");
      awaitChange(seen(), Math.min(deadline, now + 250));
    }
  }
}
