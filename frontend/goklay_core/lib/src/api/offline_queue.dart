import 'package:flutter/foundation.dart';

import 'api_client.dart';
import 'api_error.dart';

/// A write that was made with no connection and still has to happen.
@immutable
class QueuedAction {
  /// Creates a queued write.
  const QueuedAction({
    required this.id,
    required this.method,
    required this.path,
    this.body,
  });

  /// The caller's own id for this action, used to reconcile it with whatever
  /// the UI showed optimistically.
  final String id;

  /// The HTTP method, for example `POST`.
  final String method;

  /// The path, for example `/v1/partner/jobs/JOB-1/deliver`.
  final String path;

  /// The JSON body, or null.
  final Object? body;
}

/// What happened when the queue was replayed.
@immutable
class FlushReport {
  /// Creates a report.
  const FlushReport({required this.sent, required this.rejected});

  /// Actions the server accepted, in the order they were sent.
  final List<QueuedAction> sent;

  /// Actions the server refused outright, with the failure it gave.
  ///
  /// These are dropped from the queue: a 409 or a 422 will be refused just as
  /// firmly in an hour, and a queue that retries them forever never drains.
  /// They are reported so the app can tell the user what did not happen.
  final List<(QueuedAction, ApiError)> rejected;

  /// Whether anything at all moved.
  bool get isEmpty => sent.isEmpty && rejected.isEmpty;
}

/// Holds writes made offline and replays them, in order, on reconnect.
///
/// The partner app is the reason this exists. A rider loses signal in a lift,
/// a basement or half of rural Bangladesh, and still taps "collected" and
/// "delivered". Those taps cannot be lost, and they cannot be replayed out of
/// order either — the order module's state machine will refuse `delivered`
/// from an order that never reached `picked_up`.
///
/// So replay is strictly FIFO and stops at the first action that fails because
/// the network is still down, leaving it and everything after it queued.
class OfflineQueue {
  /// Creates an empty queue.
  OfflineQueue();

  final List<QueuedAction> _pending = <QueuedAction>[];

  /// The actions waiting to be sent, oldest first.
  List<QueuedAction> get pending => List<QueuedAction>.unmodifiable(_pending);

  /// Whether anything is waiting.
  bool get isEmpty => _pending.isEmpty;

  /// Adds an action to the back of the queue.
  void enqueue(QueuedAction action) => _pending.add(action);

  /// Drops everything. Called on sign-out, so one rider's unsent work is never
  /// replayed under the next rider's token.
  void clear() => _pending.clear();

  /// Replays the queue through [client], oldest first.
  ///
  /// Stops at the first action that fails for lack of a connection, leaving it
  /// queued. Actions the server actively refuses are removed and returned in
  /// [FlushReport.rejected].
  Future<FlushReport> flush(GoklayApiClient client) async {
    final List<QueuedAction> sent = <QueuedAction>[];
    final List<(QueuedAction, ApiError)> rejected =
        <(QueuedAction, ApiError)>[];

    while (_pending.isNotEmpty) {
      final QueuedAction action = _pending.first;
      try {
        await client.send(action.method, action.path, body: action.body);
        _pending.removeAt(0);
        sent.add(action);
      } on ApiError catch (error) {
        if (error.isOffline) {
          break;
        }
        _pending.removeAt(0);
        rejected.add((action, error));
      }
    }
    return FlushReport(sent: sent, rejected: rejected);
  }
}
