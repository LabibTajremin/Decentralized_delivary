import 'package:flutter/widgets.dart';

import '../api/money.dart';
import '../tokens/typography.dart';

/// Paints an amount of money.
///
/// It renders [Money.display] and nothing else. It does not format [Money.minor],
/// does not insert a currency symbol, does not group digits and does not choose
/// a decimal separator — the server did all of that, in the user's language,
/// and a second implementation here would eventually disagree with the receipt.
///
/// The widget exists precisely so that painting money is a thing you reach for
/// rather than a `Text('৳${...}')` somebody writes in a hurry.
class GoklayMoneyText extends StatelessWidget {
  /// Paints [money] in [style], defaulting to the emphasis style the design
  /// uses for figures.
  const GoklayMoneyText(this.money, {this.style, super.key});

  /// The amount, as the server sent it.
  final Money money;

  /// The style to paint it in. Defaults to [GoklayTextStyles.emphasis].
  final TextStyle? style;

  @override
  Widget build(BuildContext context) {
    return Text(
      money.display,
      style: style ?? GoklayTextStyles.emphasis,
    );
  }
}
