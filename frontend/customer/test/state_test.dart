import 'package:flutter_test/flutter_test.dart';
import 'package:goklay_core/goklay_core.dart';
import 'package:goklay_customer/src/api/api_page.dart';
import 'package:goklay_customer/src/session/session.dart';
import 'package:goklay_customer/src/session/token_storage.dart';
import 'package:goklay_customer/src/state/async_value.dart';
import 'package:goklay_customer/src/state/store.dart';

void main() {
  group('AsyncValue', () {
    test('only data has a value', () {
      expect(const AsyncLoading<int>().valueOrNull, isNull);
      expect(const AsyncData<int>(4).valueOrNull, 4);
      expect(
        AsyncFailure<int>(ApiError.offline()).valueOrNull,
        isNull,
      );
    });

    test('loading is the only state that is loading', () {
      expect(const AsyncLoading<int>().isLoading, isTrue);
      expect(const AsyncData<int>(4).isLoading, isFalse);
    });

    test('data remembers it came out of the cache', () {
      final DateTime when = DateTime.utc(2026, 9, 21);
      const ApiResponse response = ApiResponse(data: null, fromCache: true);
      final ApiPage<int> page = ApiPage<int>(
        value: 9,
        fromCache: response.fromCache,
        storedAt: when,
      );
      final AsyncData<int> data = page.toAsyncData();
      expect(data.fromCache, isTrue);
      expect(data.storedAt, when);
    });
  });

  group('Store', () {
    test('starts loading and moves to data', () async {
      final Store<int> store = Store<int>();
      expect(store.state, isA<AsyncLoading<int>>());
      await store.load(() async => const AsyncData<int>(3));
      expect(store.state.valueOrNull, 3);
      store.dispose();
    });

    test('an ApiError becomes a failure rather than escaping', () async {
      final Store<int> store = Store<int>();
      await store.load(() async => throw ApiError.offline());
      expect(store.state, isA<AsyncFailure<int>>());
      expect((store.state as AsyncFailure<int>).error.isOffline, isTrue);
      store.dispose();
    });

    test('a refresh can keep the old value on screen', () async {
      final Store<int> store = Store<int>.of(1);
      final List<bool> sawLoading = <bool>[];
      store.addListener(() => sawLoading.add(store.state.isLoading));
      await store.load(
        () async => const AsyncData<int>(2),
        keepShowingWhileLoading: true,
      );
      expect(sawLoading, <bool>[false]);
      expect(store.state.valueOrNull, 2);
      store.dispose();
    });

    test('emit replaces the state and notifies', () async {
      final Store<int> store = Store<int>();
      int notifications = 0;
      store.addListener(() => notifications += 1);
      store.emit(const AsyncData<int>(5));
      expect(notifications, 1);
      expect(store.state.valueOrNull, 5);
      store.dispose();
    });
  });

  group('ActionRunner', () {
    test('reports success and clears the previous failure', () async {
      final ActionRunner runner = ActionRunner();
      expect(await runner.run(() async => throw ApiError.offline()), isFalse);
      expect(runner.error, isNotNull);
      expect(await runner.run(() async {}), isTrue);
      expect(runner.error, isNull);
      runner.dispose();
    });

    test('a double tap cannot start a second run', () async {
      final ActionRunner runner = ActionRunner();
      int started = 0;
      final Future<bool> first = runner.run(() async {
        started += 1;
        await Future<void>.delayed(const Duration(milliseconds: 10));
      });
      final bool second = await runner.run(() async => started += 1);
      expect(second, isFalse);
      expect(await first, isTrue);
      expect(started, 1);
      runner.dispose();
    });

    test('a dismissed failure does not come back', () async {
      final ActionRunner runner = ActionRunner();
      await runner.run(() async => throw ApiError.offline());
      runner.clearError();
      expect(runner.error, isNull);
      runner.dispose();
    });
  });

  group('Session', () {
    test('restores nothing when nobody has signed in', () async {
      final Session session = Session(InMemoryTokenStorage());
      await session.restore();
      expect(session.isSignedIn, isFalse);
      expect(session.isRestored, isTrue);
      expect(session.accessToken, isNull);
      session.dispose();
    });

    test('a half-written pair is treated as signed out and cleared', () async {
      final InMemoryTokenStorage storage = InMemoryTokenStorage(
        const StoredTokens(accessToken: 'a', refreshToken: ''),
      );
      final Session session = Session(storage);
      await session.restore();
      expect(session.isSignedIn, isFalse);
      expect(await storage.read(), isNull);
      session.dispose();
    });

    test('adopting a pair persists it, signing out removes it', () async {
      final InMemoryTokenStorage storage = InMemoryTokenStorage();
      final Session session = Session(storage);
      await session.adopt(
        const StoredTokens(accessToken: 'a', refreshToken: 'r'),
      );
      expect(session.accessToken, 'a');
      expect(session.refreshToken, 'r');
      expect(await storage.read(), isNotNull);
      await session.signOut();
      expect(session.isSignedIn, isFalse);
      expect(await storage.read(), isNull);
      session.dispose();
    });
  });

  group('StoredTokens', () {
    test('both halves are needed to be complete', () {
      expect(
        const StoredTokens(accessToken: 'a', refreshToken: 'r').isComplete,
        isTrue,
      );
      expect(
        const StoredTokens(accessToken: '', refreshToken: 'r').isComplete,
        isFalse,
      );
    });

    test('reads the token pair the auth endpoints answer with', () {
      final StoredTokens tokens = StoredTokens.fromJson(<String, Object?>{
        'access_token': 'a',
        'refresh_token': 'r',
      });
      expect(tokens, const StoredTokens(accessToken: 'a', refreshToken: 'r'));
      expect(
        tokens.hashCode,
        const StoredTokens(accessToken: 'a', refreshToken: 'r').hashCode,
      );
      expect(StoredTokens.fromJson(const <String, Object?>{}).isComplete,
          isFalse);
    });
  });
}
