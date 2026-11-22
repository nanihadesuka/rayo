package rayo;

/** Where the time of a request went. */
final class Timing {
  private Timing() {}

  /** Where the time of a request went, in microseconds. */
  static final java.util.Map<String, Long> spent = new java.util.TreeMap<>();

  static void spend(String what, long began) {
    if (Thread.currentThread().getName().startsWith("rayo-")) return;
    spent.merge(what, (System.nanoTime() - began) / 1000, Long::sum);
    if (what.equals("read")) spent.merge("reads", 1L, Long::sum);
  }
}
