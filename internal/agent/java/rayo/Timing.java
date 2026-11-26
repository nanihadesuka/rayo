package rayo;

/** Where the time of a request went. */
final class Timing {
  private Timing() {}

  /** Where the time of a request went, in microseconds. */
  static volatile Thread owner; // the thread answering requests, set by Agent.main

  static final java.util.Map<String, Long> spent = new java.util.TreeMap<>();

  static void spend(String what, long began) {
    // Only the thread answering requests: another (the shutdown hook) would race the map's reader.
    if (Thread.currentThread() != owner) return;
    spent.merge(what, (System.nanoTime() - began) / 1000, Long::sum);
    if (what.equals("read")) spent.merge("reads", 1L, Long::sum);
  }
}
