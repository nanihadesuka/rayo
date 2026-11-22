package rayo;

import static rayo.Input.*;

import android.graphics.Rect;
import android.view.accessibility.AccessibilityNodeInfo;
import java.util.ArrayList;
import java.util.Comparator;
import java.util.HashSet;
import java.util.List;
import java.util.Set;
import java.util.regex.Pattern;
import java.util.regex.PatternSyntaxException;
import org.json.JSONArray;
import org.json.JSONObject;

/** A selector: every condition must hold. */
final class Selector {
  final JSONObject json;
  final Pattern id;
  final Pattern text;
  final String rawId;
  final String rawText;

  Selector(JSONObject json) throws Exception {
    this.json = json;
    rawId = json.has("id") ? json.getString("id") : null;
    rawText = json.has("text") ? json.getString("text") : null;
    id = regex(rawId);
    text = regex(rawText);
  }

  private static Pattern regex(String s) {
    if (s == null) return null;
    int flags = Pattern.CASE_INSENSITIVE | Pattern.DOTALL | Pattern.MULTILINE;
    try {
      return Pattern.compile(s, flags);
    } catch (PatternSyntaxException e) {
      return Pattern.compile(Pattern.quote(s), flags);
    }
  }

  /** Depends on one element only. */
  boolean simple() {
    for (String key : new String[] {"index", "below", "above", "leftOf", "rightOf", "containsChild", "childOf", "containsDescendants"}) {
      if (json.has(key)) return false;
    }
    return true;
  }

  Selector sub(String key) throws Exception {
    return json.has(key) ? new Selector(json.getJSONObject(key)) : null;
  }

  private boolean hit(Pattern p, String raw, CharSequence value) {
    return value != null && (raw.contentEquals(value) || p.matcher(value).matches());
  }

  private boolean flag(String key, boolean value) {
    return !json.has(key) || json.optBoolean(key) == value;
  }

  boolean self(AccessibilityNodeInfo n) {
    if (id != null) {
      String name = n.getViewIdResourceName();
      if (name == null) return false;
      int slash = name.indexOf(":id/");
      if (!hit(id, rawId, name) && !(slash >= 0 && hit(id, rawId, name.substring(slash + 4)))) return false;
    }
    if (text != null && !hit(text, rawText, n.getText()) && !hit(text, rawText, n.getContentDescription())
        && !hit(text, rawText, n.getHintText())) {
      return false;
    }
    return flag("enabled", n.isEnabled()) && flag("checked", n.isChecked())
        && flag("focused", n.isFocused()) && flag("selected", n.isSelected());
  }

  /** All matches: the innermost of nested ones, from top to bottom. */
  List<Entry> among(List<Entry> all) throws Exception {
    Set<Integer> found = new HashSet<>();
    for (int i = 0; i < all.size(); i++) {
      Entry e = all.get(i);
      if (!e.bounds.isEmpty() && self(e.node)) found.add(i);
    }
    relation(all, found);
    Set<Integer> outer = new HashSet<>();
    for (int i : found) {
      for (int p = all.get(i).parent; p >= 0; p = all.get(p).parent) outer.add(p);
    }
    List<Entry> result = new ArrayList<>();
    for (int i = 0; i < all.size(); i++) {
      if (found.contains(i) && !outer.contains(i)) result.add(all.get(i));
    }
    if (json.has("index")) result.sort(Comparator.<Entry>comparingInt(e -> e.bounds.top).thenComparingInt(e -> e.bounds.left));
    return result;
  }

  private void relation(List<Entry> all, Set<Integer> found) throws Exception {
    for (String key : new String[] {"below", "above", "leftOf", "rightOf"}) {
      Selector s = sub(key);
      if (s == null || found.isEmpty()) continue;
      Entry anchor = s.first(all);
      found.removeIf(i -> {
        if (anchor == null) return true;
        Rect b = all.get(i).bounds;
        Rect a = anchor.bounds;
        switch (key) {
          case "below":
            return b.top < a.bottom;
          case "above":
            return b.bottom > a.top;
          case "leftOf":
            return b.right > a.left;
          default:
            return b.left < a.right;
        }
      });
    }
    Selector child = sub("containsChild");
    if (child != null) {
      Set<Integer> parents = new HashSet<>();
      for (Entry e : child.among(all)) parents.add(e.parent);
      found.retainAll(parents);
    }
    Selector parent = sub("childOf");
    if (parent != null) {
      Set<Entry> parents = new HashSet<>(parent.among(all));
      found.removeIf(i -> {
        for (int p = all.get(i).parent; p >= 0; p = all.get(p).parent) {
          if (parents.contains(all.get(p))) return false;
        }
        return true;
      });
    }
    JSONArray inside = json.optJSONArray("containsDescendants");
    for (int k = 0; inside != null && k < inside.length(); k++) {
      Set<Integer> around = new HashSet<>();
      for (Entry e : new Selector(inside.getJSONObject(k)).among(all)) {
        for (int p = e.parent; p >= 0; p = all.get(p).parent) around.add(p);
      }
      found.retainAll(around);
    }
  }

  Entry first(List<Entry> all) throws Exception {
    List<Entry> found = among(all);
    int index = json.optInt("index", 0);
    if (index < 0) index += found.size();
    return index >= 0 && index < found.size() ? found.get(index) : null;
  }

  @Override
  public String toString() {
    return json.toString();
  }
}
