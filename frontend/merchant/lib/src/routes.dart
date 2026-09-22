import 'package:flutter/material.dart';
import 'package:goklay_core/goklay_core.dart';

import 'api/models/catalogue.dart';
import 'api/models/merchant.dart';
import 'api/models/order.dart';
import 'app_scope.dart';
import 'screens/account_screen.dart';
import 'screens/document_screen.dart';
import 'screens/holiday_screen.dart';
import 'screens/hours_screen.dart';
import 'screens/item_form_screen.dart';
import 'screens/order_screen.dart';
import 'screens/shop_details_screen.dart';

/// Where each merchant screen goes next.
///
/// Navigator 1.0 and ordinary pushes, the same as the customer app: the
/// deepest stack here is board → order, and a router would be a dependency
/// bought for nothing.
abstract final class MerchantRoutes {
  static Future<T?> _push<T>(BuildContext context, Widget screen) =>
      Navigator.of(context).push<T>(
        MaterialPageRoute<T>(builder: (BuildContext context) => screen),
      );

  /// Opens the details form, answering the shop the server saved.
  static Future<Merchant?> details(
    BuildContext context, {
    Merchant? shop,
  }) => _push<Merchant>(
    context,
    ShopDetailsScreen(
      shop: shop,
      onSaved: (Merchant saved) => Navigator.of(context).pop(saved),
    ),
  );

  /// Opens the opening-hours editor.
  static Future<void> hours(BuildContext context, Merchant shop) =>
      _push<void>(
        context,
        HoursScreen(
          shop: shop,
          onSaved: (Merchant _) => Navigator.of(context).pop(),
        ),
      );

  /// Opens the holiday screen.
  static Future<void> holiday(BuildContext context, Merchant shop) =>
      _push<void>(
        context,
        HolidayScreen(
          shop: shop,
          onSaved: (Merchant _) => Navigator.of(context).pop(),
        ),
      );

  /// Opens the document form, answering whether one was added.
  static Future<bool> document(BuildContext context, Merchant shop) async {
    final bool? added = await _push<bool>(
      context,
      DocumentScreen(
        shop: shop,
        onAdded: (Merchant _) => Navigator.of(context).pop(true),
      ),
    );
    return added ?? false;
  }

  /// Opens the item form, answering whether one was added.
  ///
  /// The sections are read here rather than passed in, because the form needs
  /// them and the catalogue screen holds them in a store the form does not
  /// share.
  static Future<bool> item(
    BuildContext context, {
    required String merchantId,
    required CatalogueCapabilities capabilities,
  }) async {
    final ApiPage<List<OwnerCategory>> categories = await MerchantScope.of(
      context,
    ).catalogue.categories(merchantId);
    if (!context.mounted) {
      return false;
    }
    final bool? added = await _push<bool>(
      context,
      ItemFormScreen(
        merchantId: merchantId,
        capabilities: capabilities,
        categories: categories.value,
        onAdded: (OwnerItem _) => Navigator.of(context).pop(true),
      ),
    );
    return added ?? false;
  }

  /// Opens one order.
  static Future<void> order(
    BuildContext context, {
    required String merchantId,
    required MerchantOrder order,
  }) => _push<void>(
    context,
    OrderScreen(merchantId: merchantId, orderId: order.id),
  );

  /// Opens the language picker.
  static Future<void> language(
    BuildContext context, {
    required void Function(Locale locale) onChanged,
  }) => _push<void>(context, MerchantLanguageScreen(onChanged: onChanged));
}
