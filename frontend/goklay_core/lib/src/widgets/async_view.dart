import 'package:flutter/material.dart';
import '../api/api_error.dart';
import '../l10n/goklay_strings.dart';
import '../state/async_value.dart';
import '../state/store.dart';
import '../tokens/colors.dart';
import '../tokens/dimensions.dart';
import '../tokens/typography.dart';

import 'messages.dart';

/// Paints whichever of a [Store]'s three states it is in.
///
/// Every loaded screen goes through this, which is the point: the three cases
/// are handled once, in one place, instead of being re-spelled — and
/// eventually mis-spelled — per screen. The sealed [AsyncValue] means a fourth
/// state could not be added without this failing to compile.
class AsyncView<T> extends StatelessWidget {
  /// Creates a view over [store].
  const AsyncView({
    required this.store,
    required this.builder,
    this.onRetry,
    super.key,
  });

  /// What to watch.
  final Store<T> store;

  /// Paints a loaded value.
  final Widget Function(BuildContext context, T value) builder;

  /// Called by the retry button on a failure. Omitting it hides the button,
  /// which is right for a screen whose data cannot be re-fetched on its own.
  final Future<void> Function()? onRetry;

  @override
  Widget build(BuildContext context) {
    return ListenableBuilder(
      listenable: store,
      builder: (BuildContext context, _) {
        final AsyncValue<T> state = store.state;
        return switch (state) {
          AsyncLoading<T>() => const LoadingView(),
          AsyncFailure<T>(:final ApiError error) => ErrorView(
            error: error,
            onRetry: onRetry,
          ),
          AsyncData<T>(:final T value, :final bool fromCache) => Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: <Widget>[
              if (fromCache) const CachedBanner(),
              Expanded(child: builder(context, value)),
            ],
          ),
        };
      },
    );
  }
}

/// The spinner, with the shared "loading" word beneath it.
class LoadingView extends StatelessWidget {
  /// Creates the loading view.
  const LoadingView({super.key});

  @override
  Widget build(BuildContext context) {
    final GoklayStrings strings = GoklayLocalizations.of(context);
    return Center(
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: <Widget>[
          const CircularProgressIndicator(color: GoklayColors.brand),
          const SizedBox(height: GoklaySpacing.md),
          Text(strings.loading, style: GoklayTextStyles.caption),
        ],
      ),
    );
  }
}

/// A strip above content that was served from the cache.
///
/// It exists because [GoklayApiClient] falls back to the cache when the device
/// is offline, and a customer looking at yesterday's prices deserves to be
/// told rather than left to find out at checkout.
class CachedBanner extends StatelessWidget {
  /// Creates the banner.
  const CachedBanner({super.key});

  @override
  Widget build(BuildContext context) {
    final GoklayStrings strings = GoklayLocalizations.of(context);
    return Container(
      width: double.infinity,
      color: GoklayColors.brandSubtle,
      padding: const EdgeInsets.symmetric(
        horizontal: GoklaySpacing.lg,
        vertical: GoklaySpacing.sm,
      ),
      child: Text(
        strings.showingSaved,
        style: GoklayTextStyles.caption.copyWith(
          color: GoklayColors.textPrimary,
        ),
      ),
    );
  }
}
