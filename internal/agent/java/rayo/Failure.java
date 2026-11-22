package rayo;

/** What to tell the host: the request couldn't be fulfilled. */
final class Failure extends Exception {
  Failure(String message) {
    super(message);
  }
}
