import 'dart:async';
import 'dart:convert';
import 'dart:io';

import 'package:goklay_core/goklay_core.dart';
import 'package:http/http.dart' as http;

import '../models/tracking.dart';

/// The live delivery stream.
///
/// `GET /v1/track/{orderId}` is `text/event-stream`, not JSON, so it does not
/// go through [GoklayApiClient] — that client decodes a whole body, caches it
/// and returns once, which is the opposite of what a stream needs. This owns
/// the same three concerns the shared client does (language, bearer token,
/// failure shape) and nothing more.
///
/// Nothing is cached. A stale rider position is worse than none: it would draw
/// a motorbike sitting still on a map while the real one is three streets
/// away.
class TrackingApi {
  /// Creates a tracking client.
  const TrackingApi({
    required this.baseUrl,
    required http.Client httpClient,
    this.accessToken,
    this.language,
  }) : _http = httpClient;

  /// The API root.
  final Uri baseUrl;

  /// Supplies the current access token, read per connection.
  final String? Function()? accessToken;

  /// `en` for English, null for Bengali — the same rule the shared client
  /// applies, because the status labels in each frame are composed by the
  /// server in that language.
  final String? Function()? language;

  final http.Client _http;

  /// Opens the stream for [orderId].
  ///
  /// The stream closes on its own after the frame with `live: false`, so a
  /// screen does not have to decide which statuses are final. It throws
  /// [ApiError] if the connection is refused or the server answers with a
  /// failure instead of a stream.
  Stream<TrackingSnapshot> watch(String orderId) async* {
    final String? lang = language?.call();
    final http.Request request = http.Request(
      'GET',
      baseUrl.replace(
        path: '/v1/track/$orderId',
        queryParameters: lang == null ? null : <String, String>{'lang': lang},
      ),
    )..headers['accept'] = 'text/event-stream';
    final String? token = accessToken?.call();
    if (token != null && token.isNotEmpty) {
      request.headers['authorization'] = 'Bearer $token';
    }

    final http.StreamedResponse response;
    try {
      response = await _http.send(request);
    } on SocketException {
      throw ApiError.offline();
    } on http.ClientException {
      throw ApiError.offline();
    }

    if (response.statusCode < 200 || response.statusCode >= 300) {
      throw ApiError.fromResponse(
        response.statusCode,
        _decodeOrNull(await response.stream.bytesToString()),
      );
    }

    final StringBuffer data = StringBuffer();
    await for (final String line
        in response.stream.transform(utf8.decoder).transform(const LineSplitter())) {
      if (line.isEmpty) {
        final TrackingSnapshot? frame = _frame(data.toString());
        data.clear();
        if (frame == null) {
          continue;
        }
        yield frame;
        if (!frame.live) {
          return;
        }
        continue;
      }
      if (line.startsWith('data:')) {
        data.write(line.substring('data:'.length).trimLeft());
      }
    }

    // A stream can end without the trailing blank line — a dropped connection,
    // or a server that closed straight after its last frame.
    final TrackingSnapshot? last = _frame(data.toString());
    if (last != null) {
      yield last;
    }
  }

  static TrackingSnapshot? _frame(String payload) {
    final Object? decoded = _decodeOrNull(payload);
    return decoded is Map<String, Object?>
        ? TrackingSnapshot.fromJson(decoded)
        : null;
  }

  static Object? _decodeOrNull(String body) {
    if (body.isEmpty) {
      return null;
    }
    try {
      return jsonDecode(body) as Object?;
    } on FormatException {
      return null;
    }
  }
}
