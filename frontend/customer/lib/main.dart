import 'package:flutter/widgets.dart';

import 'src/app.dart';
import 'src/environment.dart';

/// Starts the customer app.
///
/// Wiring only: it builds the environment and hands it to [GoklayCustomerApp].
/// Nothing here decides anything, which is why the widget tests drive
/// [GoklayCustomerApp] directly and this function is the one place the
/// coverage gate is allowed to skip.
void main() {
  runApp(GoklayCustomerApp(environment: CustomerEnvironment.fromCompileTime()));
}
