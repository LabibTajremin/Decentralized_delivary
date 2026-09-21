/// The foundation the GoKlay customer, merchant and partner apps are built on.
///
/// Four things live here, and they are the four an app cannot be allowed to
/// each decide for itself:
///
/// * **Tokens and theme** taken from the Figma source, so three apps look like
///   one product.
/// * **An accessibility floor** — 48dp touch targets and WCAG AA contrast —
///   enforced by tests over the real theme rather than by review.
/// * **Bengali-first localisation**, including the Bengali face the design file
///   does not contain.
/// * **The API transport**, which owns language, authentication, caching and
///   the offline queue so no screen has to.
///
/// What is deliberately absent is any business rule. There is no fee, no
/// radius, no threshold and no arithmetic on money anywhere in this package,
/// and `scripts/thin-client-lint.sh` fails the build if one appears (2.9).
library;

export 'src/a11y/contrast.dart';
export 'src/a11y/tap_target.dart';
export 'src/api/api_client.dart';
export 'src/api/api_error.dart';
export 'src/api/capability.dart';
export 'src/api/money.dart';
export 'src/api/offline_queue.dart';
export 'src/api/response_cache.dart';
export 'src/l10n/goklay_strings.dart';
export 'src/theme/goklay_theme.dart';
export 'src/tokens/colors.dart';
export 'src/tokens/dimensions.dart';
export 'src/tokens/typography.dart';
export 'src/widgets/goklay_button.dart';
export 'src/widgets/money_text.dart';
