import 'package:flutter/foundation.dart';

/// One stored response body, with the validator the server gave it.
@immutable
class CachedResponse {
  /// Creates a cache entry.
  const CachedResponse({
    required this.body,
    required this.storedAt,
    this.etag,
  });

  /// The raw response body, exactly as it arrived.
  final String body;

  /// When it was stored, so a screen can say how old what it is showing is.
  final DateTime storedAt;

  /// The server's `ETag`, replayed as `If-None-Match` on the next request so
  /// an unchanged resource costs a 304 and no body.
  final String? etag;
}

/// Where responses are kept between requests, and between launches.
///
/// This exists so the app can paint before the network answers, which the
/// performance budget requires: the home feed is expected to be on screen
/// immediately, and a rider who has lost signal still has to see the address
/// they are delivering to.
///
/// The foundation ships only [InMemoryResponseCache]. A disk-backed
/// implementation belongs to whichever app first needs reads to survive a
/// restart, and plugs in here without the transport changing.
abstract class ResponseCache {
  /// The entry for [key], or null when nothing is stored.
  Future<CachedResponse?> read(String key);

  /// Stores [value] under [key], replacing anything already there.
  Future<void> write(String key, CachedResponse value);

  /// Drops everything. Called on sign-out: one account's cached screens must
  /// never be visible to the next person who signs in on the same handset.
  Future<void> clear();
}

/// A cache that lives as long as the process does.
class InMemoryResponseCache implements ResponseCache {
  /// Creates an empty cache.
  InMemoryResponseCache();

  final Map<String, CachedResponse> _entries = <String, CachedResponse>{};

  @override
  Future<CachedResponse?> read(String key) async => _entries[key];

  @override
  Future<void> write(String key, CachedResponse value) async {
    _entries[key] = value;
  }

  @override
  Future<void> clear() async => _entries.clear();
}
