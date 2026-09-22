import 'package:flutter/widgets.dart';

import 'dependencies.dart';

/// Puts the [Dependencies] graph in scope for every screen below it.
class MerchantScope extends InheritedWidget {
  /// Creates the scope.
  const MerchantScope({
    required this.dependencies,
    required super.child,
    super.key,
  });

  /// What is in scope.
  final Dependencies dependencies;

  /// The graph above [context].
  ///
  /// It throws rather than returning null: a screen without dependencies
  /// cannot do anything useful, and failing at the first frame in a test is
  /// cheaper than a null that surfaces three taps later.
  static Dependencies of(BuildContext context) {
    final MerchantScope? scope = context
        .dependOnInheritedWidgetOfExactType<MerchantScope>();
    assert(scope != null, 'No MerchantScope above this widget');
    return scope!.dependencies;
  }

  @override
  bool updateShouldNotify(MerchantScope oldWidget) =>
      oldWidget.dependencies != dependencies;
}
