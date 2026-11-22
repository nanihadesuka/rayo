package rayo;

import android.graphics.Rect;
import android.view.accessibility.AccessibilityNodeInfo;

/** One element, in drawing order. */
final class Entry {
  final AccessibilityNodeInfo node;
  final Rect bounds;
  final int parent;

  Entry(AccessibilityNodeInfo node, Rect bounds, int parent) {
    this.node = node;
    this.bounds = bounds;
    this.parent = parent;
  }

  String label() {
    if (node.getText() != null && node.getText().length() > 0) return node.getText().toString();
    return node.getContentDescription() == null ? "" : node.getContentDescription().toString();
  }
}
