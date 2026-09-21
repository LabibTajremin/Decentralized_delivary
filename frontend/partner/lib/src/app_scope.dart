import 'package:flutter/widgets.dart';

import 'dependencies.dart';

/// Puts the [Dependencies] graph in scope for every screen below it.
class PartnerScope extends InheritedWidget {
  /// Creates the scope.
  const PartnerScope({
    required this.dependencies,
    required super.child,
    super.key,
  });

  /// What is in scope.
  final Dependencies dependencies;

  /// The graph above [context].
  static Dependencies of(BuildContext context) {
    final PartnerScope? scope = context
        .dependOnInheritedWidgetOfExactType<PartnerScope>();
    assert(scope != null, 'No PartnerScope above this widget');
    return scope!.dependencies;
  }

  @override
  bool updateShouldNotify(PartnerScope oldWidget) =>
      oldWidget.dependencies != dependencies;
}
