import 'package:goklay_core/goklay_core.dart';

import '../models/merchant.dart';

/// Registering a shop and keeping its details, hours and holiday.
class MerchantApi {
  /// Creates the API against [client].
  const MerchantApi(this._client);

  final GoklayApiClient _client;

  /// What the registration form must ask for, per shop type.
  Future<ApiPage<RegistrationRequirements>> requirements() async {
    final ApiResponse response = await _client.get(
      '/v1/merchants/registration-requirements',
    );
    return ApiPage<RegistrationRequirements>.of(
      response,
      RegistrationRequirements.fromJson(response.asObject),
    );
  }

  /// The signed-in owner's shop.
  Future<ApiPage<Merchant>> me() async {
    final ApiResponse response = await _client.get('/v1/merchants/me');
    return ApiPage<Merchant>.of(response, Merchant.fromJson(response.asObject));
  }

  /// Registers a shop.
  ///
  /// The area, district and division are not sent: P02 resolves them from the
  /// coordinate, and they are the D3 boundary the shop will forever be served
  /// within — not something a form should be able to state.
  Future<Merchant> register({
    required String name,
    required String type,
    required String phone,
    required String line1,
    required double lat,
    required double lng,
    String email = '',
    String line2 = '',
  }) async {
    final ApiResponse response = await _client.send(
      'POST',
      '/v1/merchants',
      body: _details(
        name: name,
        type: type,
        phone: phone,
        line1: line1,
        lat: lat,
        lng: lng,
        email: email,
        line2: line2,
      ),
    );
    return Merchant.fromJson(response.asObject);
  }

  /// Replaces the shop's details.
  Future<Merchant> updateDetails({
    required String name,
    required String type,
    required String phone,
    required String line1,
    required double lat,
    required double lng,
    String email = '',
    String line2 = '',
  }) async {
    final ApiResponse response = await _client.send(
      'PATCH',
      '/v1/merchants/me',
      body: _details(
        name: name,
        type: type,
        phone: phone,
        line1: line1,
        lat: lat,
        lng: lng,
        email: email,
        line2: line2,
      ),
    );
    return Merchant.fromJson(response.asObject);
  }

  /// Uploads one of the documents the shop's type requires.
  Future<Merchant> addDocument({
    required String kind,
    required String number,
    required String fileUrl,
  }) async {
    final ApiResponse response = await _client.send(
      'POST',
      '/v1/merchants/me/documents',
      body: <String, Object?>{
        'kind': kind,
        'number': number,
        'file_url': fileUrl,
      },
    );
    return Merchant.fromJson(response.asObject);
  }

  /// Submits the shop for an admin's review.
  ///
  /// Whether this is allowed is `Merchant.canSubmit`, which the server decides
  /// from the documents it actually holds. The app offers the button on that
  /// flag and lets the server refuse anyway.
  Future<Merchant> submitForReview() async {
    final ApiResponse response = await _client.send(
      'POST',
      '/v1/merchants/me/submit',
    );
    return Merchant.fromJson(response.asObject);
  }

  /// Replaces the opening hours.
  ///
  /// Keyed by weekday number with Sunday as `"0"`, each window `HH:MM-HH:MM`.
  /// The server rejects overlapping windows and ones that wrap past midnight;
  /// the app sends what was typed and shows the refusal.
  Future<Merchant> setHours(Map<String, List<String>> days) async {
    final ApiResponse response = await _client.send(
      'PUT',
      '/v1/merchants/me/hours',
      body: <String, Object?>{'days': days},
    );
    return Merchant.fromJson(response.asObject);
  }

  /// Closes the shop for a holiday, or reopens it.
  ///
  /// A null [until] closes it indefinitely; [clear] reopens it. Being on
  /// holiday is what takes the shop off every customer's list, which is why
  /// it is one call and not a local flag.
  Future<Merchant> setHoliday({
    DateTime? until,
    String reason = '',
    bool clear = false,
  }) async {
    final ApiResponse response = await _client.send(
      'PUT',
      '/v1/merchants/me/holiday',
      body: clear
          ? <String, Object?>{}
          : <String, Object?>{
              if (until != null) 'until': until.toUtc().toIso8601String(),
              'reason': reason,
            },
    );
    return Merchant.fromJson(response.asObject);
  }

  static Map<String, Object?> _details({
    required String name,
    required String type,
    required String phone,
    required String line1,
    required double lat,
    required double lng,
    required String email,
    required String line2,
  }) => <String, Object?>{
    'name': name,
    'type': type,
    'phone': phone,
    'email': email,
    'line1': line1,
    'line2': line2,
    'lat': lat,
    'lng': lng,
  };
}
