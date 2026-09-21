import 'package:flutter/foundation.dart';
import '../state/async_value.dart';
import 'api_client.dart';


/// A parsed read, carrying whether it came out of the cache.
///
/// Reads go through [GoklayApiClient.get], which falls back to the cache when
/// the device is offline. That fact has to reach the screen — a list of shops
/// stored yesterday is worth painting, and worth labelling — so every read
/// returns this rather than the bare value, and [toAsyncData] carries it
/// straight into the store.
@immutable
class ApiPage<T> {
  /// Creates a page.
  const ApiPage({required this.value, required this.fromCache, this.storedAt});

  /// Wraps [value] with the freshness of the [response] it was parsed from.
  factory ApiPage.of(ApiResponse response, T value) => ApiPage<T>(
    value: value,
    fromCache: response.fromCache,
    storedAt: response.storedAt,
  );

  /// The parsed body.
  final T value;

  /// Whether this was served from the cache rather than the network.
  final bool fromCache;

  /// When it was cached. Null when [fromCache] is false.
  final DateTime? storedAt;

  /// The state a [Store] should hold.
  AsyncData<T> toAsyncData() =>
      AsyncData<T>(value, fromCache: fromCache, storedAt: storedAt);
}
