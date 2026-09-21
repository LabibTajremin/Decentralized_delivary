import 'package:flutter/material.dart';
import 'package:goklay_core/goklay_core.dart';

import '../api/models/cart.dart';

/// The receipt, printed exactly as the server composed it.
///
/// Every row arrives with its label and its amount already worked out, in
/// order. This widget does not know which rows exist, what `expansion_surcharge`
/// means, or how any figure was arrived at — and that is what keeps the app's
/// idea of the bill from ever disagreeing with the one the customer is charged
/// (2.9).
class ReceiptView extends StatelessWidget {
  /// Creates a receipt.
  const ReceiptView({required this.rows, this.total, this.totalLabel, super.key});

  /// The rows, in the order they print.
  final List<ReceiptRow> rows;

  /// The total, drawn heavier under a rule. Null when the caller's last row
  /// already is the total.
  final Money? total;

  /// What to call the total. Null uses nothing, since a total line without a
  /// label is still unambiguous under a receipt.
  final String? totalLabel;

  @override
  Widget build(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: <Widget>[
        for (final ReceiptRow row in rows)
          Padding(
            padding: const EdgeInsets.symmetric(vertical: GoklaySpacing.xxs),
            child: Row(
              children: <Widget>[
                Expanded(
                  child: Text(row.label, style: GoklayTextStyles.body),
                ),
                GoklayMoneyText(row.amount, style: GoklayTextStyles.body),
              ],
            ),
          ),
        if (total != null) ...<Widget>[
          const Divider(color: GoklayColors.border),
          Row(
            children: <Widget>[
              Expanded(
                child: Text(
                  totalLabel ?? '',
                  style: GoklayTextStyles.emphasis,
                ),
              ),
              GoklayMoneyText(total!),
            ],
          ),
        ],
      ],
    );
  }
}
