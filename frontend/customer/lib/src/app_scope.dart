import 'package:flutter/widgets.dart';

import 'dependencies.dart';

/// Puts the [Dependencies] graph in scope for every screen below it.
///
/// An [InheritedWidget] rather than a global, so a widget test can pump one
/// screen with a fake-backed graph and there is nothing left over to reset
/// afterwards.
class AppScope extends InheritedWidget {
  /// Creates the scope.
  const AppScope({
    required this.dependencies,
    required super.child,
    super.key,
  });

  /// What is in scope.
  final Dependencies dependencies;

  /// The graph above [context].
  ///
  /// It throws rather than returning null when there is none: a screen without
  /// dependencies cannot do anything useful, and failing at the first frame in
  /// a test is far cheaper than a null that surfaces three taps later.
  static Dependencies of(BuildContext context) {
    final AppScope? scope = context
        .dependOnInheritedWidgetOfExactType<AppScope>();
    assert(scope != null, 'No AppScope above this widget');
    return scope!.dependencies;
  }

  @override
  bool updateShouldNotify(AppScope oldWidget) =>
      oldWidget.dependencies != dependencies;
}
