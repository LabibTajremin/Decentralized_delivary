import 'package:flutter/material.dart';
import 'package:goklay_core/goklay_core.dart';

import '../api/models/merchant.dart';
import '../app_scope.dart';
import '../l10n/merchant_strings.dart';

/// Adding one of the documents the shop's type requires.
///
/// The kinds offered are `Merchant.missingDocuments` — the server's list of
/// what is still wanted, not a local subtraction of uploaded from required.
///
/// There is no file picker. This build has no camera or storage plugin, so
/// the form takes the location of a file that has already been uploaded;
/// recorded in `docs/design-gaps.md` along with the rest of the platform work
/// P19 did not invent.
class DocumentScreen extends StatefulWidget {
  /// Creates the form over [shop].
  const DocumentScreen({required this.shop, required this.onAdded, super.key});

  /// The shop the document belongs to.
  final Merchant shop;

  /// Called with the shop the server answered with.
  final void Function(Merchant shop) onAdded;

  @override
  State<DocumentScreen> createState() => _DocumentScreenState();
}

class _DocumentScreenState extends State<DocumentScreen> {
  final ActionRunner _runner = ActionRunner();
  final TextEditingController _number = TextEditingController();
  final TextEditingController _file = TextEditingController();
  late String _kind = widget.shop.missingDocuments.isEmpty
      ? (widget.shop.requiredDocuments.isEmpty
            ? ''
            : widget.shop.requiredDocuments.first)
      : widget.shop.missingDocuments.first;

  @override
  void dispose() {
    _number.dispose();
    _file.dispose();
    _runner.dispose();
    super.dispose();
  }

  Future<void> _add() async {
    Merchant? saved;
    final bool ok = await _runner.run(() async {
      saved = await MerchantScope.of(context).merchant.addDocument(
        kind: _kind,
        number: _number.text.trim(),
        fileUrl: _file.text.trim(),
      );
    });
    if (ok && saved != null && mounted) {
      widget.onAdded(saved!);
    }
  }

  @override
  Widget build(BuildContext context) {
    final MerchantStrings strings = MerchantLocalizations.of(context);
    final List<String> kinds = widget.shop.missingDocuments.isEmpty
        ? widget.shop.requiredDocuments
        : widget.shop.missingDocuments;
    return GoklayScaffold(
      title: strings.addDocument,
      body: ListenableBuilder(
        listenable: _runner,
        builder: (BuildContext context, _) => ListView(
          padding: const EdgeInsets.all(GoklaySpacing.lg),
          children: <Widget>[
            for (final String kind in kinds)
              GoklayTapTarget(
                onTap: _runner.isBusy
                    ? null
                    : () => setState(() => _kind = kind),
                semanticLabel: kind,
                child: Padding(
                  padding: const EdgeInsets.symmetric(
                    vertical: GoklaySpacing.xxs,
                  ),
                  child: Row(
                    children: <Widget>[
                      Icon(
                        _kind == kind
                            ? Icons.check_circle
                            : Icons.circle_outlined,
                        size: 20,
                        color: _kind == kind
                            ? GoklayColors.brand
                            : GoklayColors.textSecondary,
                      ),
                      const SizedBox(width: GoklaySpacing.sm),
                      Text(kind, style: GoklayTextStyles.body),
                    ],
                  ),
                ),
              ),
            LabelledField(
              label: strings.documentNumber,
              controller: _number,
            ),
            LabelledField(label: strings.documentFile, controller: _file),
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
              label: strings.addDocument,
              expand: true,
              onPressed: _runner.isBusy || _kind.isEmpty ? null : _add,
            ),
          ],
        ),
      ),
    );
  }
}
