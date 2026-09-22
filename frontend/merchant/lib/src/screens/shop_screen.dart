import 'package:flutter/material.dart';
import 'package:goklay_core/goklay_core.dart';

import '../api/models/merchant.dart';
import '../app_scope.dart';
import '../dependencies.dart';
import '../l10n/merchant_strings.dart';
import '../widgets/cards.dart';

/// The shop's own record: its details, its documents, and where it stands in
/// the approval workflow.
///
/// Three server-decided fields drive the whole screen. `missing_documents`
/// says what is still wanted — the app does not subtract the uploaded list
/// from the required one. `can_submit` says whether the submit button is
/// live. `open_status` and `review_note` are sentences the server composed,
/// and are printed as they arrived.
class ShopScreen extends StatefulWidget {
  /// Creates the screen.
  const ShopScreen({
    required this.onEditDetails,
    required this.onHours,
    required this.onHoliday,
    required this.onAddDocument,
    this.showBack = true,
    super.key,
  });

  /// Opens the details form.
  final void Function(Merchant shop) onEditDetails;

  /// Opens the hours editor.
  final void Function(Merchant shop) onHours;

  /// Opens the holiday screen.
  final void Function(Merchant shop) onHoliday;

  /// Opens the document form. Answers true when one was added.
  final Future<bool> Function(Merchant shop) onAddDocument;

  /// Whether to draw a back button.
  final bool showBack;

  @override
  State<ShopScreen> createState() => _ShopScreenState();
}

class _ShopScreenState extends State<ShopScreen> {
  final Store<Merchant> _shop = Store<Merchant>();
  final ActionRunner _runner = ActionRunner();

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) => _load());
  }

  @override
  void dispose() {
    _shop.dispose();
    _runner.dispose();
    super.dispose();
  }

  Future<void> _load() async {
    final Dependencies dependencies = MerchantScope.of(context);
    await _shop.load(() async {
      final ApiPage<Merchant> page = await dependencies.merchant.me();
      return page.toAsyncData();
    });
  }

  Future<void> _submit() async {
    final Dependencies dependencies = MerchantScope.of(context);
    await _runner.run(() async {
      final Merchant updated = await dependencies.merchant.submitForReview();
      _shop.emit(AsyncData<Merchant>(updated));
    });
  }

  Future<void> _addDocument(Merchant shop) async {
    final bool added = await widget.onAddDocument(shop);
    if (added && mounted) {
      await _load();
    }
  }

  @override
  Widget build(BuildContext context) {
    final MerchantStrings strings = MerchantLocalizations.of(context);
    return GoklayScaffold(
      title: strings.shopTitle,
      showBack: widget.showBack,
      body: ListenableBuilder(
        listenable: _runner,
        builder: (BuildContext context, _) => AsyncView<Merchant>(
          store: _shop,
          onRetry: _load,
          builder: (BuildContext context, Merchant shop) =>
              _body(strings, shop),
        ),
      ),
    );
  }

  Widget _body(MerchantStrings strings, Merchant shop) {
    return ListView(
      children: <Widget>[
        Padding(
          padding: const EdgeInsets.all(GoklaySpacing.lg),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: <Widget>[
              Text(shop.name, style: GoklayTextStyles.title),
              const SizedBox(height: GoklaySpacing.xxs),
              Text(
                shop.openStatus,
                style: GoklayTextStyles.body.copyWith(
                  color: shop.isOpenNow
                      ? GoklayColors.brand
                      : GoklayColors.textSecondary,
                ),
              ),
              const SizedBox(height: GoklaySpacing.xxs),
              Text(
                shop.singleLine,
                style: GoklayTextStyles.caption.copyWith(
                  color: GoklayColors.textSecondary,
                ),
              ),
              if (shop.hasReviewNote) ...<Widget>[
                const SizedBox(height: GoklaySpacing.sm),
                Text(
                  shop.reviewNote,
                  style: GoklayTextStyles.body.copyWith(
                    color: GoklayColors.danger,
                  ),
                ),
              ],
              if (shop.isAwaitingReview) ...<Widget>[
                const SizedBox(height: GoklaySpacing.sm),
                Text(strings.awaitingReview, style: GoklayTextStyles.body),
              ],
              if (shop.holiday != null) ...<Widget>[
                const SizedBox(height: GoklaySpacing.sm),
                Text(
                  strings.onHoliday,
                  style: GoklayTextStyles.body.copyWith(
                    color: GoklayColors.danger,
                  ),
                ),
              ],
            ],
          ),
        ),
        SettingRow(
          label: strings.shopRow,
          onTap: () => widget.onEditDetails(shop),
        ),
        SettingRow(
          label: strings.hoursRow,
          onTap: () => widget.onHours(shop),
        ),
        SettingRow(
          label: strings.holidayRow,
          onTap: () => widget.onHoliday(shop),
        ),
        SectionHeading(strings.documents),
        for (final MerchantDocument document in shop.documents)
          GoklayCard(
            child: Row(
              children: <Widget>[
                Expanded(
                  child: Text(document.kind, style: GoklayTextStyles.body),
                ),
                Text(document.number, style: GoklayTextStyles.caption),
              ],
            ),
          ),
        if (shop.missingDocuments.isNotEmpty) ...<Widget>[
          SectionHeading(strings.stillNeeded),
          for (final String kind in shop.missingDocuments)
            Padding(
              padding: const EdgeInsets.symmetric(
                horizontal: GoklaySpacing.lg,
                vertical: GoklaySpacing.xxs,
              ),
              child: Text(
                kind,
                style: GoklayTextStyles.body.copyWith(
                  color: GoklayColors.danger,
                ),
              ),
            ),
        ],
        Padding(
          padding: const EdgeInsets.all(GoklaySpacing.lg),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: <Widget>[
              GoklayButton(
                label: strings.addDocument,
                variant: GoklayButtonVariant.outlined,
                expand: true,
                onPressed: _runner.isBusy ? null : () => _addDocument(shop),
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
              if (shop.canSubmit) ...<Widget>[
                const SizedBox(height: GoklaySpacing.sm),
                GoklayButton(
                  label: strings.submitForReview,
                  expand: true,
                  onPressed: _runner.isBusy ? null : _submit,
                ),
              ],
            ],
          ),
        ),
      ],
    );
  }
}
