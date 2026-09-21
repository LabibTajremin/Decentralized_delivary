import 'package:flutter/material.dart';
import 'package:goklay_core/goklay_core.dart';

import '../api/api_page.dart';
import '../api/models/notification.dart';
import '../app_scope.dart';
import '../dependencies.dart';
import '../l10n/customer_strings.dart';
import '../state/store.dart';
import '../widgets/async_view.dart';
import '../widgets/cards.dart';
import '../widgets/messages.dart';
import '../widgets/scaffold.dart';

/// The notification history.
///
/// The design has no frame for this; P14 built the endpoint, and
/// `docs/design-gaps.md` records why the screen exists anyway. There is
/// nothing to design around: both the title and the body are composed by the
/// server, in the customer's language, so the screen is a list of what it was
/// given.
class NotificationsScreen extends StatefulWidget {
  /// Creates the list.
  const NotificationsScreen({super.key});

  @override
  State<NotificationsScreen> createState() => _NotificationsScreenState();
}

class _NotificationsScreenState extends State<NotificationsScreen> {
  final Store<List<AppNotification>> _notifications =
      Store<List<AppNotification>>();

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) => _load());
  }

  @override
  void dispose() {
    _notifications.dispose();
    super.dispose();
  }

  Future<void> _load() async {
    final Dependencies dependencies = AppScope.of(context);
    await _notifications.load(() async {
      final ApiPage<List<AppNotification>> page = await dependencies
          .notifications
          .list();
      return page.toAsyncData();
    });
  }

  @override
  Widget build(BuildContext context) {
    final CustomerStrings strings = CustomerLocalizations.of(context);
    return CustomerScaffold(
      title: strings.notifications,
      body: AsyncView<List<AppNotification>>(
        store: _notifications,
        onRetry: _load,
        builder:
            (BuildContext context, List<AppNotification> notifications) =>
                notifications.isEmpty
                ? EmptyView(message: strings.noNotifications)
                : ListView(
                    children: <Widget>[
                      for (final AppNotification notification
                          in notifications)
                        GoklayCard(
                          child: Column(
                            crossAxisAlignment: CrossAxisAlignment.start,
                            children: <Widget>[
                              Text(
                                notification.title,
                                style: GoklayTextStyles.emphasis,
                              ),
                              Text(
                                notification.body,
                                style: GoklayTextStyles.body,
                              ),
                              if (notification.didFail)
                                Text(
                                  strings.notificationFailed,
                                  style: GoklayTextStyles.caption.copyWith(
                                    color: GoklayColors.danger,
                                  ),
                                ),
                            ],
                          ),
                        ),
                    ],
                  ),
      ),
    );
  }
}
