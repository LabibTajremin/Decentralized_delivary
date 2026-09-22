import 'package:flutter/foundation.dart';
import '../api/api_error.dart';

/// What a screen has to show right now: nothing yet, something, or a failure.
///
/// Sealed so that a screen has to handle all three. The pattern this replaces
/// is a widget with three nullable fields and an `if (error != null)` somebody
/// forgets, which is how a spinner ends up spinning forever over a failed
/// request.
@immutable
sealed class AsyncValue<T> {
  const AsyncValue();

  /// Whether a request is in flight.
  bool get isLoading => this is AsyncLoading<T>;

  /// The value, or null when there is not one yet.
  T? get valueOrNull => switch (this) {
    AsyncData<T>(:final T value) => value,
    _ => null,
  };
}

/// Waiting for the first response.
@immutable
final class AsyncLoading<T> extends AsyncValue<T> {
  /// Creates the loading state.
  const AsyncLoading();
}

/// A value to show.
@immutable
final class AsyncData<T> extends AsyncValue<T> {
  /// Creates a loaded state.
  const AsyncData(this.value, {this.fromCache = false, this.storedAt});

  /// What to paint.
  final T value;

  /// Whether this came out of the cache rather than the network, so a screen
  /// can say it is showing something it has not just confirmed.
  final bool fromCache;

  /// When a cached value was stored. Null when [fromCache] is false.
  final DateTime? storedAt;
}

/// The request failed and there is nothing cached to fall back on.
@immutable
final class AsyncFailure<T> extends AsyncValue<T> {
  /// Creates a failed state.
  const AsyncFailure(this.error);

  /// What went wrong, carrying the server's own message where there is one.
  final ApiError error;
}
