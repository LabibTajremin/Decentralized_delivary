import 'package:flutter/material.dart';

import 'api/models/account.dart';
import 'api/models/cart.dart';
import 'api/models/catalogue.dart';
import 'api/models/discovery.dart';
import 'api/models/order.dart';
import 'screens/account_screen.dart';
import 'screens/add_address_screen.dart';
import 'screens/addresses_screen.dart';
import 'screens/item_screen.dart';
import 'screens/language_screen.dart';
import 'screens/notifications_screen.dart';
import 'screens/order_screen.dart';
import 'screens/payment_screen.dart';
import 'screens/place_order_screen.dart';
import 'screens/profile_screen.dart';
import 'screens/placeholder_screen.dart';
import 'screens/review_cart_screen.dart';
import 'screens/review_screen.dart';
import 'screens/security_screen.dart';
import 'screens/shop_reviews_screen.dart';
import 'screens/shop_screen.dart';
import 'screens/support_screen.dart';
import 'screens/tracking_screen.dart';

/// Where each screen goes next.
///
/// Navigator 1.0 and ordinary pushes, with no routing package and no URL
/// table. A phone app whose deepest stack is shop → item → cart → review →
/// place order does not need a router, and the screens stay testable on their
/// own: each takes callbacks, and this file is the only place that knows what
/// those callbacks push.
abstract final class CustomerRoutes {
  static Future<T?> _push<T>(BuildContext context, Widget screen) =>
      Navigator.of(context).push<T>(
        MaterialPageRoute<T>(builder: (BuildContext context) => screen),
      );

  /// Opens a shop's menu.
  static Future<void> shop(
    BuildContext context,
    DiscoveryMerchant merchant, {
    required VoidCallback onCartChanged,
  }) {
    return _push<void>(
      context,
      ShopScreen(
        merchant: merchant,
        onItemSelected: (PublicItem selected) =>
            item(context, selected, onAdded: onCartChanged),
        onReviews: () => shopReviews(context, merchant),
      ),
    );
  }

  /// Opens one item's detail and add-to-cart form.
  static Future<void> item(
    BuildContext context,
    PublicItem selected, {
    required VoidCallback onAdded,
  }) {
    return _push<void>(
      context,
      ItemScreen(
        item: selected,
        onAdded: () {
          onAdded();
          Navigator.of(context).pop();
        },
      ),
    );
  }

  /// Opens a shop's reviews.
  static Future<void> shopReviews(
    BuildContext context,
    DiscoveryMerchant merchant,
  ) {
    return _push<void>(
      context,
      ShopReviewsScreen(
        merchantId: merchant.id,
        merchantName: merchant.name,
      ),
    );
  }

  /// Opens the review-cart step.
  static Future<void> reviewCart(BuildContext context, Cart cart) {
    return _push<void>(
      context,
      ReviewCartScreen(
        cart: cart,
        onContinue: (Cart bound, Address address) =>
            placeOrder(context, bound, address),
      ),
    );
  }

  /// Opens the confirm-and-place step.
  static Future<void> placeOrder(
    BuildContext context,
    Cart cart,
    Address address,
  ) {
    return _push<void>(
      context,
      PlaceOrderScreen(
        cart: cart,
        address: address,
        onPlaced: (Order placed) => order(context, placed.id),
      ),
    );
  }

  /// Opens one order.
  static Future<void> order(BuildContext context, String orderId) {
    return _push<void>(
      context,
      OrderScreen(
        orderId: orderId,
        onTrack: (Order tracked) => tracking(context, tracked.id),
        onPay: (Order unpaid) => payment(context, unpaid.id),
        onReview: (Order finished) => review(context, finished),
        onSupport: (Order subject) => support(context, orderId: subject.id),
      ),
    );
  }

  /// Opens live tracking.
  static Future<void> tracking(BuildContext context, String orderId) =>
      _push<void>(context, TrackingScreen(orderId: orderId));

  /// Opens the payment screen.
  static Future<void> payment(BuildContext context, String orderId) =>
      _push<void>(context, PaymentScreen(orderId: orderId));

  /// Opens the review form.
  static Future<void> review(BuildContext context, Order reviewed) =>
      _push<void>(context, ReviewScreen(order: reviewed));

  /// Opens support, with the form when an order is named.
  static Future<void> support(BuildContext context, {String? orderId}) =>
      _push<void>(context, SupportScreen(orderId: orderId));

  /// Opens the profile.
  static Future<void> profile(BuildContext context) =>
      _push<void>(context, const ProfileScreen());

  /// Opens the address book.
  static Future<void> addresses(BuildContext context) => _push<void>(
    context,
    AddressesScreen(onAdd: () => addAddress(context)),
  );

  /// Opens the add-address form, answering whether one was saved.
  static Future<bool> addAddress(BuildContext context) async {
    final bool? saved = await _push<bool>(
      context,
      AddAddressScreen(
        onSaved: (Address _) => Navigator.of(context).pop(true),
      ),
    );
    return saved ?? false;
  }

  /// Opens the device list.
  ///
  /// Signing out from here pops back to the root first. The app's two halves
  /// are swapped underneath the navigator, so a screen left on the stack would
  /// keep a signed-out customer looking at their own device list.
  static Future<void> security(
    BuildContext context, {
    required VoidCallback onSignedOut,
  }) => _push<void>(
    context,
    SecurityScreen(
      onSignedOut: () {
        Navigator.of(context).popUntil((Route<void> route) => route.isFirst);
        onSignedOut();
      },
    ),
  );

  /// Opens the language picker.
  static Future<void> language(
    BuildContext context, {
    required void Function(Locale locale) onChanged,
  }) => _push<void>(context, LanguageScreen(onChanged: onChanged));

  /// Opens the notification history.
  static Future<void> notifications(BuildContext context) =>
      _push<void>(context, const NotificationsScreen());

  /// Opens one of the screens with no backend behind it.
  static Future<void> placeholder(
    BuildContext context,
    PlaceholderScreen screen,
  ) => _push<void>(context, PlaceholderScreenView(screen: screen));
}
