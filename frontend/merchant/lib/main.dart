import 'package:flutter/widgets.dart';

import 'src/app.dart';
import 'src/environment.dart';

/// Starts the merchant app.
///
/// Wiring only. The widget tests drive [GoklayMerchantApp] directly, which is
/// why this function is the one place the coverage gate never sees.
void main() {
  runApp(GoklayMerchantApp(environment: MerchantEnvironment.fromCompileTime()));
}
