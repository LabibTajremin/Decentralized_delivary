/// The foundation the GoKlay customer, merchant and partner apps are built on.
///
/// What lives here is everything three apps would otherwise each decide for
/// themselves:
///
/// * **Tokens and theme** taken from the Figma source, so three apps look like
///   one product.
/// * **An accessibility floor** — 48dp touch targets and WCAG AA contrast —
///   enforced by tests over the real theme rather than by review.
/// * **Bengali-first localisation**, including the Bengali face the design file
///   does not contain.
/// * **The API transport**, which owns language, authentication, caching and
///   the offline queue so no screen has to — with the auth and profile calls
///   every app makes, and the models they answer with.
/// * **The state primitives and the shared screen furniture**: a sealed
///   `AsyncValue`, a `Store`, an `ActionRunner`, a session, and the widgets
///   that paint loading, failure, emptiness, a receipt and a rating. P18
///   wrote them for the customer app; P19 moved them here rather than
///   triplicating them, because three copies of "what does a failed request
///   look like" become three different answers.
///
/// What is deliberately absent is any business rule. There is no fee, no
/// radius, no threshold and no arithmetic on money anywhere in this package,
/// and `scripts/thin-client-lint.sh` fails the build if one appears (2.9).
library;

export 'src/a11y/contrast.dart';
export 'src/a11y/tap_target.dart';
export 'src/auth/otp_screen.dart';
export 'src/auth/phone_sign_in_screen.dart';
export 'src/auth/splash_screen.dart';
export 'src/auth/verification_result_screen.dart';
export 'src/api/account_api.dart';
export 'src/api/api_client.dart';
export 'src/api/api_error.dart';
export 'src/api/api_page.dart';
export 'src/api/auth_api.dart';
export 'src/api/capability.dart';
export 'src/api/json.dart';
export 'src/api/money.dart';
export 'src/api/offline_queue.dart';
export 'src/api/response_cache.dart';
export 'src/l10n/goklay_strings.dart';
export 'src/models/account.dart';
export 'src/models/auth.dart';
export 'src/models/receipt.dart';
export 'src/session/session.dart';
export 'src/session/token_storage.dart';
export 'src/state/async_value.dart';
export 'src/state/store.dart';
export 'src/theme/goklay_theme.dart';
export 'src/tokens/colors.dart';
export 'src/tokens/dimensions.dart';
export 'src/tokens/typography.dart';
export 'src/widgets/async_view.dart';
export 'src/widgets/goklay_button.dart';
export 'src/widgets/messages.dart';
export 'src/widgets/money_text.dart';
export 'src/widgets/quantity_stepper.dart';
export 'src/widgets/receipt_view.dart';
export 'src/widgets/scaffold.dart';
export 'src/widgets/star_rating.dart';
