import '../models/account.dart';

import 'api_client.dart';
import 'api_page.dart';

/// The profile and address book.
class AccountApi {
  /// Creates the API against [client].
  const AccountApi(this._client);

  final GoklayApiClient _client;

  /// The signed-in customer.
  Future<ApiPage<Profile>> profile() async {
    final ApiResponse response = await _client.get('/v1/me');
    return ApiPage<Profile>.of(response, Profile.fromJson(response.asObject));
  }

  /// Changes the name, email or language. Fields left null are not sent, so a
  /// screen that edits one cannot blank another.
  Future<Profile> updateProfile({
    String? name,
    String? email,
    String? language,
  }) async {
    final ApiResponse response = await _client.send(
      'PATCH',
      '/v1/me',
      body: <String, Object?>{
        'name': ?name,
        'email': ?email,
        'language': ?language,
      },
    );
    return Profile.fromJson(response.asObject);
  }

  /// Every address the customer has saved.
  Future<ApiPage<List<Address>>> addresses() async {
    final ApiResponse response = await _client.get('/v1/me/addresses');
    return ApiPage<List<Address>>.of(response, _addresses(response));
  }

  /// Saves a new address.
  ///
  /// The area, district and division are not sent: the server resolves them
  /// from the coordinate (P02), and a client that guessed would eventually
  /// place an address in a division the order could not be served from.
  Future<Address> addAddress({
    required String label,
    required String recipientName,
    required String recipientPhone,
    required String line1,
    required double lat,
    required double lng,
    String line2 = '',
    String instructions = '',
    bool makeDefault = false,
  }) async {
    final ApiResponse response = await _client.send(
      'POST',
      '/v1/me/addresses',
      body: <String, Object?>{
        'label': label,
        'recipient_name': recipientName,
        'recipient_phone': recipientPhone,
        'line1': line1,
        'line2': line2,
        'instructions': instructions,
        'lat': lat,
        'lng': lng,
        'make_default': makeDefault,
      },
    );
    return Address.fromJson(response.asObject);
  }

  /// Removes an address.
  Future<void> deleteAddress(String id) =>
      _client.send('DELETE', '/v1/me/addresses/$id');

  /// Makes an address the one checkout starts with.
  Future<Address> makeDefault(String id) async {
    final ApiResponse response = await _client.send(
      'POST',
      '/v1/me/addresses/$id/default',
    );
    return Address.fromJson(response.asObject);
  }

  static List<Address> _addresses(ApiResponse response) {
    final Object? raw = response.asObject['addresses'];
    if (raw is! List<Object?>) {
      return const <Address>[];
    }
    return <Address>[
      for (final Object? entry in raw)
        if (entry is Map<String, Object?>) Address.fromJson(entry),
    ];
  }
}
