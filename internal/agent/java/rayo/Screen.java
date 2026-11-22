package rayo;

import static rayo.Timing.*;
import static rayo.Ui.*;

import android.graphics.Rect;
import android.view.accessibility.AccessibilityNodeInfo;
import java.util.ArrayList;
import java.util.List;

/** What is on screen. */
final class Screen {
  private Screen() {}

  /** The screen as Android remembers it: almost free, but it may be a moment old. */
  static List<Entry> remembered() {
    long began = System.nanoTime();
    List<Entry> all = new ArrayList<>();
    for (AccessibilityNodeInfo root : roots(true)) collect(root, -1, screen(), all);
    spend("cache", began);
    return all;
  }

  static List<Entry> entries(boolean system) {
    long began = System.nanoTime();
    clearCache();
    List<Entry> all = new ArrayList<>();
    Rect screen = screen();
    for (AccessibilityNodeInfo root : roots(true)) collect(root, -1, screen, all);
    if (system) {
      for (AccessibilityNodeInfo root : roots(false)) collect(root, -1, screen, all);
    }
    spend("read", began);
    spent.put("nodes", (long) all.size());
    return all;
  }

  static List<Entry> entries() {
    return entries(true);
  }

  static void collect(AccessibilityNodeInfo n, int parent, Rect screen, List<Entry> all) {
    if (!n.isVisibleToUser()) return;
    Rect b = new Rect();
    n.getBoundsInScreen(b);
    if (!b.intersect(screen)) b.setEmpty();
    int me = all.size();
    all.add(new Entry(n, b, parent));
    for (int i = 0; i < n.getChildCount(); i++) {
      AccessibilityNodeInfo child = PREFETCH == 0 ? n.getChild(i) : n.getChild(i, PREFETCH);
      if (child != null) collect(child, me, screen, all);
    }
  }

  /** Only what a flow can refer to. */
  static String tree() {
    StringBuilder text = new StringBuilder();
    List<Entry> all = entries();
    int[] depth = new int[all.size()];
    for (int i = 0; i < all.size(); i++) {
      Entry e = all.get(i);
      AccessibilityNodeInfo n = e.node;
      depth[i] = e.parent < 0 ? 1 : depth[e.parent];
      if (e.parent < 0) text.append('[').append(n.getPackageName()).append("]\n");
      List<String> parts = new ArrayList<>();
      if (n.getViewIdResourceName() != null) parts.add("id: " + n.getViewIdResourceName());
      if (n.getText() != null && n.getText().length() > 0) parts.add("text: " + n.getText());
      if (n.getContentDescription() != null && n.getContentDescription().length() > 0) parts.add("desc: " + n.getContentDescription());
      if (parts.isEmpty()) continue;
      if (!n.isEnabled()) parts.add("disabled");
      if (n.isFocused()) parts.add("focused");
      if (n.isChecked()) parts.add("checked");
      if (n.isSelected()) parts.add("selected");
      for (int k = 0; k < depth[i]; k++) text.append("  ");
      text.append(String.join(" | ", parts).replace('\n', ' ')).append("  ").append(e.bounds.toShortString()).append('\n');
      depth[i]++;
    }
    return text.toString();
  }
}
