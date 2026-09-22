import 'package:goklay_core/goklay_core.dart';

import '../models/job.dart';
import '../models/ledger.dart';
import '../models/partner.dart';

/// Everything a rider's app asks the backend for.
class PartnerApi {
  /// Creates the API against [client].
  const PartnerApi(this._client);

  final GoklayApiClient _client;

  /// The caller's own partner record.
  Future<ApiPage<DeliveryPartner>> me() async {
    final ApiResponse response = await _client.get('/v1/partner');
    return ApiPage<DeliveryPartner>.of(
      response,
      DeliveryPartner.fromJson(response.asObject),
    );
  }

  /// Signs up to carry orders.
  ///
  /// **D1 from the partner's side:** a delivery partner may work anywhere in
  /// Bangladesh, so there is no area to register in and nothing to approve
  /// against a map. Where they can work is decided every time they report a
  /// location. Registering twice is not an error — it is somebody tapping a
  /// button again, and they get the partner they already are.
  Future<DeliveryPartner> register({
    required String name,
    required String phone,
    String vehicle = '',
  }) async {
    final ApiResponse response = await _client.send(
      'POST',
      '/v1/partner',
      body: <String, Object?>{
        'name': name,
        'phone': phone,
        if (vehicle.isNotEmpty) 'vehicle': vehicle,
      },
    );
    return DeliveryPartner.fromJson(response.asObject);
  }

  /// Goes on or off shift.
  ///
  /// Only `offline` and `available` may be sent. `busy` is what being at the
  /// concurrent limit is called and the server sets it; a rider who could
  /// declare it would have a way to stay in the pool while refusing every
  /// offer.
  Future<DeliveryPartner> setAvailability(String availability) async {
    final ApiResponse response = await _client.send(
      'PUT',
      '/v1/partner/availability',
      body: <String, Object?>{'availability': availability},
    );
    return DeliveryPartner.fromJson(response.asObject);
  }

  /// Chooses long-distance, short-distance or both (D4).
  Future<DeliveryPartner> setPreference(String preference) async {
    final ApiResponse response = await _client.send(
      'PUT',
      '/v1/partner/preference',
      body: <String, Object?>{'preference': preference},
    );
    return DeliveryPartner.fromJson(response.asObject);
  }

  /// Reports where the rider is.
  ///
  /// Called often as they move. It writes one row and nothing else: the feed
  /// is computed on read rather than pushed on write, because a partner who
  /// is driving is not looking at their phone.
  Future<DeliveryPartner> reportLocation({
    required double lat,
    required double lng,
  }) async {
    final ApiResponse response = await _client.send(
      'PUT',
      '/v1/partner/location',
      body: <String, Object?>{'lat': lat, 'lng': lng},
    );
    return DeliveryPartner.fromJson(response.asObject);
  }

  /// What the rider should be looking at right now (ALG-08).
  Future<ApiPage<PartnerFeed>> feed() async {
    final ApiResponse response = await _client.get('/v1/partner/feed');
    return ApiPage<PartnerFeed>.of(
      response,
      PartnerFeed.fromJson(response.asObject),
    );
  }

  /// The rider's own deliveries. [live] asks for what they are carrying.
  Future<ApiPage<DeliveryJobList>> jobs({
    bool live = true,
    int limit = 20,
    int offset = 0,
  }) async {
    final ApiResponse response = await _client.get(
      '/v1/partner/jobs',
      query: <String, String>{
        if (live) 'live': 'true',
        'limit': '$limit',
        'offset': '$offset',
      },
    );
    return ApiPage<DeliveryJobList>.of(
      response,
      DeliveryJobList.fromJson(response.asObject),
    );
  }

  /// Takes an offered delivery.
  ///
  /// Two riders cannot hold the same order: the second one gets a `409`, and
  /// the app shows that refusal rather than pretending the job is theirs.
  Future<DeliveryJob> accept(String jobId) => _move(jobId, 'accept');

  /// Passes on an offered delivery.
  Future<DeliveryJob> decline(String jobId) => _move(jobId, 'decline');

  /// Says the goods are in the bag.
  Future<DeliveryJob> collect(String jobId) => _move(jobId, 'collect');

  /// Says they have been handed over.
  Future<DeliveryJob> deliver(String jobId) => _move(jobId, 'deliver');

  /// Says the delivery could not be completed.
  ///
  /// The reason is required, and it is the rider's own words: a customer and
  /// a shop left with an abandoned delivery have nothing else to go on.
  Future<DeliveryJob> fail({
    required String jobId,
    required String reason,
  }) => _move(jobId, 'fail', body: <String, Object?>{'reason': reason});

  /// The rider's cash-on-delivery ledger.
  Future<ApiPage<Ledger>> ledger() async {
    final ApiResponse response = await _client.get('/v1/partner/cod');
    return ApiPage<Ledger>.of(response, Ledger.fromJson(response.asObject));
  }

  Future<DeliveryJob> _move(
    String jobId,
    String action, {
    Object? body,
  }) async {
    final ApiResponse response = await _client.send(
      'POST',
      '/v1/partner/jobs/$jobId/$action',
      body: body,
    );
    return DeliveryJob.fromJson(response.asObject);
  }
}
