import 'dart:async';

import 'package:flutter/material.dart';
import 'package:goklay_core/goklay_core.dart';

import '../api/models/tracking.dart';
import '../app_scope.dart';
import '../l10n/customer_strings.dart';

/// `10__Order Tracking` / `11__Order Tracking` (`1:10633`, `1:10691`) — the
/// before-and-after of a rider being assigned.
///
/// The stream decides which of the two is showing: while
/// [TrackingSnapshot.partner] is null nobody has collected the order, and the
/// screen says so. It ends on the frame with `live: false`, so the app never
/// has to hold its own list of which statuses are terminal — the order state
/// machine can gain a state without this screen learning about it.
///
/// No map tile is drawn. There is no maps plugin in this build and no map key
/// in the environment; the rider's coordinates are shown as the position the
/// server last reported. Making that a map is platform work, not a change to
/// what the app knows.
class TrackingScreen extends StatefulWidget {
  /// Creates the screen for [orderId].
  const TrackingScreen({required this.orderId, super.key});

  /// Which order to follow.
  final String orderId;

  @override
  State<TrackingScreen> createState() => _TrackingScreenState();
}

class _TrackingScreenState extends State<TrackingScreen> {
  StreamSubscription<TrackingSnapshot>? _subscription;
  TrackingSnapshot? _snapshot;
  ApiError? _error;
  bool _finished = false;

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) => _watch());
  }

  @override
  void dispose() {
    unawaited(_subscription?.cancel());
    super.dispose();
  }

  void _watch() {
    setState(() {
      _error = null;
      _finished = false;
    });
    _subscription = AppScope.of(context).tracking.watch(widget.orderId).listen(
      (TrackingSnapshot snapshot) {
        if (mounted) {
          setState(() => _snapshot = snapshot);
        }
      },
      onError: (Object error) {
        if (mounted) {
          setState(
            () => _error = error is ApiError
                ? error
                : ApiError.unreadable(0),
          );
        }
      },
      onDone: () {
        if (mounted) {
          setState(() => _finished = true);
        }
      },
    );
  }

  @override
  Widget build(BuildContext context) {
    final CustomerStrings strings = CustomerLocalizations.of(context);
    final ApiError? error = _error;
    final TrackingSnapshot? snapshot = _snapshot;
    return GoklayScaffold(
      title: strings.trackingTitle,
      body: error != null
          ? ErrorView(
              error: error,
              onRetry: () async => _watch(),
            )
          : snapshot == null
          ? const Center(
              child: CircularProgressIndicator(color: GoklayColors.brand),
            )
          : _body(strings, snapshot),
    );
  }

  Widget _body(CustomerStrings strings, TrackingSnapshot snapshot) {
    final TrackingPartner? partner = snapshot.partner;
    return Padding(
      padding: const EdgeInsets.all(GoklaySpacing.lg),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: <Widget>[
          Text(snapshot.statusLabel, style: GoklayTextStyles.title),
          const SizedBox(height: GoklaySpacing.lg),
          if (partner == null)
            Text(strings.awaitingRider, style: GoklayTextStyles.body)
          else ...<Widget>[
            Text(partner.name, style: GoklayTextStyles.emphasis),
            Text(partner.vehicle, style: GoklayTextStyles.caption),
            Text(partner.phone, style: GoklayTextStyles.caption),
            const SizedBox(height: GoklaySpacing.sm),
            Text(
              '${partner.lat}, ${partner.lng}',
              style: GoklayTextStyles.caption.copyWith(
                color: GoklayColors.textSecondary,
              ),
            ),
          ],
          const Spacer(),
          if (_finished)
            Text(
              strings.trackingEnded,
              style: GoklayTextStyles.caption.copyWith(
                color: GoklayColors.textSecondary,
              ),
            ),
        ],
      ),
    );
  }
}
