import 'package:flutter/foundation.dart';
import '../api/api_error.dart';

import 'async_value.dart';

/// One screen's worth of loaded state.
///
/// This is the app's whole state-management approach, and it is deliberately
/// [ChangeNotifier] and nothing else. 2.9 asks for one approach across all
/// three apps; Flutter ships one that a `ListenableBuilder` renders without a
/// package, and a thin client that mostly paints server responses does not
/// have the kind of state a larger framework exists to tame. Beating this
/// needs a reason written down, not a preference.
class Store<T> extends ChangeNotifier {
  /// Creates a store that has not loaded yet.
  Store();

  /// Creates a store already holding [value]. For tests and for screens handed
  /// data by the screen that pushed them.
  Store.of(T value) : _state = AsyncData<T>(value);

  AsyncValue<T> _state = AsyncLoading<T>();

  /// What the screen should paint.
  AsyncValue<T> get state => _state;

  /// Replaces the state and rebuilds anything listening.
  void emit(AsyncValue<T> next) {
    _state = next;
    notifyListeners();
  }

  /// Runs [fetch] and moves the store through loading to data or failure.
  ///
  /// [keepShowingWhileLoading] leaves the current value on screen instead of
  /// replacing it with a spinner, which is what a pull-to-refresh wants: the
  /// list stays, the indicator turns, nothing flashes.
  Future<void> load(
    Future<AsyncData<T>> Function() fetch, {
    bool keepShowingWhileLoading = false,
  }) async {
    if (!keepShowingWhileLoading) {
      emit(AsyncLoading<T>());
    }
    try {
      emit(await fetch());
    } on ApiError catch (error) {
      emit(AsyncFailure<T>(error));
    }
  }
}

/// A one-shot action — place the order, save the address, submit the review.
///
/// Separate from [Store] because a screen usually has both: a page that is
/// loaded and a button that is submitting. Folding the two together is how a
/// failed save blanks a screen that was showing perfectly good data.
class ActionRunner extends ChangeNotifier {
  bool _isBusy = false;
  ApiError? _error;

  /// Whether the action is in flight. Buttons disable on this.
  bool get isBusy => _isBusy;

  /// The last failure, or null. Cleared when the action is retried.
  ApiError? get error => _error;

  /// Runs [action], returning true when it succeeded.
  ///
  /// Refuses to start a second run while one is in flight, so a double tap on
  /// "place order" cannot place two.
  Future<bool> run(Future<void> Function() action) async {
    if (_isBusy) {
      return false;
    }
    _isBusy = true;
    _error = null;
    notifyListeners();
    try {
      await action();
      return true;
    } on ApiError catch (error) {
      _error = error;
      return false;
    } finally {
      _isBusy = false;
      notifyListeners();
    }
  }

  /// Forgets the last failure, so a dismissed banner does not come back.
  void clearError() {
    _error = null;
    notifyListeners();
  }
}
