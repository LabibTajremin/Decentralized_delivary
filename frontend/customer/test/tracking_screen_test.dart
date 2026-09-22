import 'dart:async';
import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:goklay_core/goklay_core.dart';
import 'package:goklay_customer/src/dependencies.dart';
import 'package:goklay_customer/src/environment.dart';
import 'package:goklay_customer/src/l10n/customer_strings.dart';
import 'package:goklay_customer/src/screens/tracking_screen.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';

import 'support/harness.dart';

/// A tracking client whose stream the test feeds frame by frame.
class _Feed {
  /// Broadcast so a reconnect can listen again: a single-subscription stream
  /// would throw on the second `listen`, which is not what a reconnect does.
  final StreamController<List<int>> controller =
      StreamController<List<int>>.broadcast();
  int status = 200;

  http.Client get client => MockClient.streaming((
    http.BaseRequest _,
    http.ByteStream _,
  ) async => http.StreamedResponse(controller.stream, status));

  void send(Map<String, Object?> frame) =>
      controller.add(utf8.encode('data: ${jsonEncode(frame)}\n\n'));

  Future<void> close() => controller.close();
}

Map<String, Object?> frame({
  String status = 'accepted',
  bool live = true,
  bool withPartner = false,
}) => <String, Object?>{
  'order_id': 'ord-1',
  'status': status,
  'status_label': withPartner ? 'পথে আছে' : 'গ্রহণ করা হয়েছে',
  'live': live,
  if (withPartner)
    'partner': <String, Object?>{
      'id': 'ptn-1',
      'name': 'করিম',
      'phone': '01811111111',
      'vehicle': 'bike',
      'lat': 23.75,
      'lng': 90.38,
    },
};

void main() {
  const CustomerStringsBn bn = CustomerStringsBn();

  Dependencies depsOver(http.Client client) => Dependencies(
    environment: CustomerEnvironment(apiBaseUrl: Uri.parse('http://api.test')),
    httpClient: client,
  );

  Future<void> pumpTracking(
    WidgetTester tester,
    Dependencies dependencies,
  ) => pumpScreen(
    tester,
    const TrackingScreen(orderId: 'ord-1'),
    dependencies: dependencies,
  );

  testWidgets('waits, then says nobody has collected it yet', (
    WidgetTester tester,
  ) async {
    final _Feed feed = _Feed();
    addTearDown(feed.close);
    final Dependencies dependencies = depsOver(feed.client);
    await pumpTracking(tester, dependencies);
    expect(find.byType(CircularProgressIndicator), findsOneWidget);

    feed.send(frame());
    await tester.pump();
    await tester.pump();
    expect(find.text('গ্রহণ করা হয়েছে'), findsOneWidget);
    expect(find.text(bn.awaitingRider), findsOneWidget);

    await tester.pump();
  });

  testWidgets('shows the rider once one is carrying it', (
    WidgetTester tester,
  ) async {
    final _Feed feed = _Feed();
    addTearDown(feed.close);
    final Dependencies dependencies = depsOver(feed.client);
    await pumpTracking(tester, dependencies);
    feed.send(frame(status: 'picked_up', withPartner: true));
    await tester.pump();
    await tester.pump();
    expect(find.text('করিম'), findsOneWidget);
    expect(find.text('bike'), findsOneWidget);
    expect(find.text('23.75, 90.38'), findsOneWidget);
    await tester.pump();
  });

  testWidgets('the last frame ends the stream and the screen says so', (
    WidgetTester tester,
  ) async {
    final _Feed feed = _Feed();
    addTearDown(feed.close);
    final Dependencies dependencies = depsOver(feed.client);
    await pumpTracking(tester, dependencies);
    feed.send(frame(status: 'delivered', live: false));
    // The generator ends by returning out of its `await for`, and cancelling
    // the underlying subscription runs on the real event loop rather than the
    // fake one a pump drives — so let it, then pump to see the result.
    await tester.runAsync(
      () => Future<void>.delayed(const Duration(milliseconds: 20)),
    );
    await tester.pump();
    expect(find.text(bn.trackingEnded), findsOneWidget);
    await tester.pump();
  });

  testWidgets('a refusal is shown, and retrying reconnects', (
    WidgetTester tester,
  ) async {
    // The failure body has to end for the error to be readable, so this one
    // answers with a stream that is already complete rather than the feed.
    int connections = 0;
    final http.Client refusing = MockClient.streaming((
      http.BaseRequest _,
      http.ByteStream _,
    ) async {
      connections += 1;
      return http.StreamedResponse(
        Stream<List<int>>.value(
          utf8.encode(
            jsonEncode(<String, Object?>{
              'error': <String, Object?>{
                'code': 'not_found',
                'message': 'অর্ডার পাওয়া যায়নি',
              },
            }),
          ),
        ),
        404,
      );
    });
    final Dependencies dependencies = depsOver(refusing);
    await pumpTracking(tester, dependencies);
    await tester.pump();
    await tester.pump();
    expect(find.text('অর্ডার পাওয়া যায়নি'), findsOneWidget);
    await tester.tap(find.bySemanticsLabel(const GoklayStringsBn().retry));
    await tester.pump();
    await tester.pump();
    await tester.pump();
    expect(connections, 2);
  });

  testWidgets('leaving the screen cancels the subscription', (
    WidgetTester tester,
  ) async {
    final _Feed feed = _Feed();
    addTearDown(feed.close);
    final Dependencies dependencies = depsOver(feed.client);
    await pumpTracking(tester, dependencies);
    feed.send(frame());
    await tester.pump();
    await tester.pumpWidget(const SizedBox.shrink());
    await tester.pump();
    // A frame after the screen is gone must not rebuild anything.
    feed.send(frame(status: 'picked_up', withPartner: true));
    await tester.pump();
  });
}
