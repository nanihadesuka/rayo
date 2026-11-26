package rayo;

import static rayo.Input.*;
import static rayo.Shell.*;

/** Turns the device's animations off, and back as they were. */
final class Animations {
  private Animations() {}

  static final String[] SCALES = {
    "window_animation_scale", "transition_animation_scale", "animator_duration_scale"
  };

  static volatile String[] savedScales;
  static Thread quieting;

  /** In the background; launching waits for it. */
  static void noAnimations() {
    // A second call would save the zeros the first one set, and the device would never get its own back.
    if (quieting != null || savedScales != null) return;
    quieting = new Thread(() -> {
      try {
        StringBuilder get = new StringBuilder();
        StringBuilder put = new StringBuilder();
        for (String s : SCALES) {
          get.append("settings get global ").append(s).append(';');
          put.append("settings put global ").append(s).append(" 0 & ");
        }
        String[] old = shell(get.toString()).trim().split("\\s+");
        if (old.length != SCALES.length) return; // unreadable, so left alone
        savedScales = old;
        shell(put + "wait");
      } catch (Exception e) {
        // Left as they are: slower, not wrong.
      }
    }, "rayo-quiet");
    quieting.start();
  }

  static void awaitQuiet() throws Exception {
    if (quieting != null) quieting.join(5000);
    quieting = null;
  }

  static void restoreAnimations() throws Exception {
    if (savedScales == null) return;
    StringBuilder undo = new StringBuilder();
    for (int i = 0; i < SCALES.length; i++) {
      String old = savedScales[i];
      undo.append(old.equals("null") ? "settings delete global " + SCALES[i] : "settings put global " + SCALES[i] + " " + old);
      undo.append(';');
    }
    savedScales = null;
    shell(undo.toString());
  }
}
