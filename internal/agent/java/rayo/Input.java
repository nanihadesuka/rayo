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
    long down = SystemClock.uptimeMillis();
    touch(down, MotionEvent.ACTION_DOWN, x, y);
    for (long t; (t = SystemClock.uptimeMillis() - down) < ms; SystemClock.sleep(8)) {
      float f = (float) t / ms;
      touch(down, MotionEvent.ACTION_MOVE, x + (x2 - x) * f, y + (y2 - y) * f);
    }
    touch(down, MotionEvent.ACTION_MOVE, x2, y2);
    touch(down, MotionEvent.ACTION_UP, x2, y2);
    settle(80, 1500); // a fling
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
    return field.performAction(AccessibilityNodeInfo.ACTION_SET_TEXT, args);
  }

  static boolean editable(AccessibilityNodeInfo n) {
    return n.isEnabled() && n.getActionList().contains(AccessibilityNodeInfo.AccessibilityAction.ACTION_SET_TEXT);
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
    for (int i = 0; i < n.getChildCount(); i++) {
      AccessibilityNodeInfo child = n.getChild(i);
      AccessibilityNodeInfo found = child == null ? null : inside(child);
      if (found != null) return found;
    }
    return null;
  }

  static AccessibilityNodeInfo focusedField(long deadline) throws Exception {
    while (true) {
      long since = seen();
      clearCache();
      AccessibilityNodeInfo n = ui.findFocus(AccessibilityNodeInfo.FOCUS_INPUT);
      if (n != null && editable(n)) return n;
      if (SystemClock.uptimeMillis() >= deadline) throw new Failure("no text field is focused");
      awaitChange(since, deadline);
    }
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

  static boolean keyboardShown() {
    for (AccessibilityWindowInfo w : ui.getWindows()) {
      if (w.getType() == AccessibilityWindowInfo.TYPE_INPUT_METHOD) return true;
    }
    return false;
  }

  static void hideKeyboard(long deadline) throws Exception {
    settle(60, 300); // it may be opening
    if (!keyboardShown()) return;
    key(KeyEvent.KEYCODE_BACK);
    while (keyboardShown()) {
      if (SystemClock.uptimeMillis() >= deadline) throw new Failure("the keyboard didn't close");
      awaitChange(seen(), deadline);
    }
  }
}
