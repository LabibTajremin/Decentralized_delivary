import 'package:flutter/widgets.dart';

import 'src/app.dart';
import 'src/environment.dart';

/// Starts the partner app.
///
/// Wiring only. The widget tests drive [GoklayPartnerApp] directly.
void main() {
  runApp(GoklayPartnerApp(environment: PartnerEnvironment.fromCompileTime()));
}
