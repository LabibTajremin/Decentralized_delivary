import 'package:flutter/material.dart';
import 'package:goklay_core/goklay_core.dart';

import '../api/models/merchant.dart';
import '../app_scope.dart';
import '../l10n/merchant_strings.dart';

/// The opening hours, one field per weekday.
///
/// Windows go to the server exactly as typed. It rejects overlaps and windows
/// that wrap past midnight — a shop trading until 2am enters `22:00-24:00`
/// and `00:00-02:00` on the next day — and the refusal is shown in its own
/// words. A second validator here would be a second opinion, and the one that
/// matters is the one that decides whether a customer sees the shop.
class HoursScreen extends StatefulWidget {
  /// Creates the editor over [shop].
  const HoursScreen({required this.shop, required this.onSaved, super.key});

  /// The shop whose hours these are.
  final Merchant shop;

  /// Called with the shop the server answered with.
  final void Function(Merchant shop) onSaved;

  /// Weekday keys as the API uses them: Sunday is `"0"`.
  static const List<String> weekdays = <String>['0', '1', '2', '3', '4', '5', '6'];

  /// The names to show beside each field, in the caller's language.
  static List<String> dayNames(MerchantStrings strings) =>
      strings.languageTag == 'en'
      ? const <String>[
          'Sunday',
          'Monday',
          'Tuesday',
          'Wednesday',
          'Thursday',
          'Friday',
          'Saturday',
        ]
      : const <String>[
          'রবিবার',
          'সোমবার',
          'মঙ্গলবার',
          'বুধবার',
          'বৃহস্পতিবার',
          'শুক্রবার',
          'শনিবার',
        ];

  @override
  State<HoursScreen> createState() => _HoursScreenState();
}

class _HoursScreenState extends State<HoursScreen> {
  final ActionRunner _runner = ActionRunner();
  late final Map<String, TextEditingController> _days =
      <String, TextEditingController>{
        for (final String day in HoursScreen.weekdays)
          day: TextEditingController(
            text: (widget.shop.hours[day] ?? const <String>[]).join(', '),
          ),
      };

  @override
  void dispose() {
    for (final TextEditingController controller in _days.values) {
      controller.dispose();
    }
    _runner.dispose();
    super.dispose();
  }

  /// A day left blank is a day the shop is closed, so it is left out rather
  /// than sent as an empty list.
  Map<String, List<String>> get _typed => <String, List<String>>{
    for (final MapEntry<String, TextEditingController> entry in _days.entries)
      if (entry.value.text.trim().isNotEmpty)
        entry.key: <String>[
          for (final String window in entry.value.text.split(','))
            if (window.trim().isNotEmpty) window.trim(),
        ],
  };

  Future<void> _save() async {
    Merchant? saved;
    final bool ok = await _runner.run(() async {
      saved = await MerchantScope.of(context).merchant.setHours(_typed);
    });
    if (ok && saved != null && mounted) {
      widget.onSaved(saved!);
    }
  }

  @override
  Widget build(BuildContext context) {
    final MerchantStrings strings = MerchantLocalizations.of(context);
    final List<String> names = HoursScreen.dayNames(strings);
    return GoklayScaffold(
      title: strings.hoursTitle,
      body: ListenableBuilder(
        listenable: _runner,
        builder: (BuildContext context, _) => ListView(
          padding: const EdgeInsets.all(GoklaySpacing.lg),
          children: <Widget>[
            Text(strings.hoursHelp, style: GoklayTextStyles.caption),
            for (int index = 0; index < HoursScreen.weekdays.length; index++)
              LabelledField(
                label: names[index],
                controller: _days[HoursScreen.weekdays[index]]!,
              ),
            if (_runner.error != null)
              Padding(
                padding: const EdgeInsets.only(top: GoklaySpacing.sm),
                child: Text(
                  ErrorView.messageFor(context, _runner.error!),
                  style: GoklayTextStyles.caption.copyWith(
                    color: GoklayColors.danger,
                  ),
                ),
              ),
            const SizedBox(height: GoklaySpacing.lg),
            GoklayButton(
              label: strings.save,
              expand: true,
              onPressed: _runner.isBusy ? null : _save,
            ),
          ],
        ),
      ),
    );
  }
}
