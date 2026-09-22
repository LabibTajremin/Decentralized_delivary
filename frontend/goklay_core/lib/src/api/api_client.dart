import 'dart:convert';
import 'dart:io';

import 'package:flutter/foundation.dart';
import 'package:flutter/widgets.dart' show Locale;
import 'package:http/http.dart' as http;

import '../l10n/goklay_strings.dart';
import 'api_error.dart';
import 'response_cache.dart';

/// A decoded response, and whether it came off the network or out of the cache.
@immutable
class ApiResponse {
  /// Creates a response.
  const ApiResponse({
    required this.data,
    required this.fromCache,
    this.storedAt,
  });

  /// The decoded JSON body: a `Map`, a `List`, or null for an empty body.
  final Object? data;

  /// Whether this was served from [ResponseCache] rather than the server.
  /// A screen showing cached content can say so.
  final bool fromCache;

  /// When a cached body was stored. Null when [fromCache] is false.
  final DateTime? storedAt;

  /// [data] as a JSON object, or an empty map when the body was not one.
  Map<String, Object?> get asObject {
    final Object? value = data;
    return value is Map<String, Object?> ? value : const <String, Object?>{};
  }
}

/// The HTTP transport every GoKlay app talks to the backend through.
///
/// It owns the three things that would otherwise be repeated, and eventually
/// forgotten, at every call site:
///
/// * **Language.** Bengali is the default, so `lang=en` is attached only when
///   the user has chosen English (1.4).
/// * **Authentication.** The access token is read at send time from a
///   callback, never captured at construction, so a refresh is picked up
///   without rebuilding the client.
/// * **Freshness.** GETs carry `If-None-Match` and store the server's `ETag`,
///   so an unchanged screen costs a 304. When the network is gone entirely, a
///   GET falls back to the cache rather than failing — the app has to open on
///   a plane, in a lift, and in a village with one bar of signal.
///
/// It deliberately does *not* retry, back off, or refresh tokens on its own.
/// Those are policies with product consequences, and hiding them inside a
/// transport is how an app ends up hammering a failing server.
class GoklayApiClient {
  /// Creates a client against [baseUrl].
  ///
  /// [httpClient] is injected so tests never open a socket. [accessToken] is
  /// called on every request; return null when nobody is signed in.
  GoklayApiClient({
    required this.baseUrl,
    required http.Client httpClient,
    ResponseCache? cache,
    this.locale = const Locale('bn'),
    this.accessToken,
  }) : _http = httpClient,
       _cache = cache ?? InMemoryResponseCache();

  /// The API root, for example `https://api.goklay.com`.
  final Uri baseUrl;

  /// The language requests are made in. Bengali unless set otherwise;
  /// mutable because the user can change it without signing out.
  Locale locale;

  /// Supplies the current access token, called fresh on every request so a
  /// refresh lands without the client being rebuilt.
  final String? Function()? accessToken;

  final http.Client _http;
  final ResponseCache _cache;

  /// The cache backing GET requests, exposed so sign-out can clear it.
  ResponseCache get cache => _cache;

  /// Reads [path], preferring the network and falling back to the cache when
  /// the device is offline.
  ///
  /// Throws [ApiError] when the server answers with a failure, or when the
  /// request fails with nothing cached to fall back on.
  Future<ApiResponse> get(String path, {Map<String, String>? query}) async {
    final Uri uri = _uri(path, query);
    final String key = uri.toString();
    final CachedResponse? cached = await _cache.read(key);

    final http.Response response;
    try {
      response = await _http.get(
        uri,
        headers: _headers(ifNoneMatch: cached?.etag),
      );
    } on SocketException {
      return _offlineFallback(cached);
    } on http.ClientException {
      return _offlineFallback(cached);
    }

    if (response.statusCode == HttpStatus.notModified && cached != null) {
      return ApiResponse(
        data: _decode(cached.body),
        fromCache: true,
        storedAt: cached.storedAt,
      );
    }
    if (_isSuccess(response.statusCode)) {
      final String? etag = response.headers['etag'];
      final String body = _bodyText(response);
      await _cache.write(
        key,
        CachedResponse(body: body, storedAt: DateTime.now(), etag: etag),
      );
      return ApiResponse(data: _decode(body), fromCache: false);
    }
    throw ApiError.fromResponse(
      response.statusCode,
      _decodeOrNull(_bodyText(response)),
    );
  }

  /// Reads [path] from the cache alone, without touching the network.
  ///
  /// This is the first half of stale-while-revalidate: a screen paints this
  /// immediately, then calls [get] and repaints when the fresh body lands.
  Future<ApiResponse?> cached(String path, {Map<String, String>? query}) async {
    final CachedResponse? entry = await _cache.read(_uri(path, query).toString());
    if (entry == null) {
      return null;
    }
    return ApiResponse(
      data: _decode(entry.body),
      fromCache: true,
      storedAt: entry.storedAt,
    );
  }

  /// Sends a write. Writes are never cached and never fall back.
  ///
  /// A failed write has to surface: silently serving a stale body in its place
  /// would tell the user their order was placed when it was not.
  Future<ApiResponse> send(
    String method,
    String path, {
    Object? body,
    Map<String, String>? query,
  }) async {
    final http.Request request = http.Request(method, _uri(path, query))
      ..headers.addAll(_headers());
    if (body != null) {
      request.headers['content-type'] = 'application/json; charset=utf-8';
      request.body = jsonEncode(body);
    }

    final http.Response response;
    try {
      response = await http.Response.fromStream(await _http.send(request));
    } on SocketException {
      throw ApiError.offline();
    } on http.ClientException {
      throw ApiError.offline();
    }

    final String responseBody = _bodyText(response);
    if (_isSuccess(response.statusCode)) {
      return ApiResponse(data: _decodeOrNull(responseBody), fromCache: false);
    }
    throw ApiError.fromResponse(
      response.statusCode,
      _decodeOrNull(responseBody),
    );
  }

  /// Releases the underlying HTTP client.
  void close() => _http.close();

  ApiResponse _offlineFallback(CachedResponse? cached) {
    if (cached == null) {
      throw ApiError.offline();
    }
    return ApiResponse(
      data: _decode(cached.body),
      fromCache: true,
      storedAt: cached.storedAt,
    );
  }

  Uri _uri(String path, Map<String, String>? query) {
    final Map<String, String> parameters = <String, String>{...?query};
    final String? lang = GoklayLocalizations.languageQueryValue(locale);
    if (lang != null) {
      parameters['lang'] = lang;
    }
    return baseUrl.replace(
      path: path,
      queryParameters: parameters.isEmpty ? null : parameters,
    );
  }

  Map<String, String> _headers({String? ifNoneMatch}) {
    final Map<String, String> headers = <String, String>{
      'accept': 'application/json',
    };
    final String? token = accessToken?.call();
    if (token != null && token.isNotEmpty) {
      headers['authorization'] = 'Bearer $token';
    }
    if (ifNoneMatch != null) {
      headers['if-none-match'] = ifNoneMatch;
    }
    return headers;
  }

  static bool _isSuccess(int status) => status >= 200 && status < 300;

  /// Reads a response body as UTF-8, whatever the server labelled it.
  ///
  /// `package:http` decodes [http.Response.body] using the charset in the
  /// Content-Type header and falls back to **latin1** when there is none. Our
  /// backend sends `application/json` with no charset, and JSON is UTF-8 by
  /// definition (RFC 8259 s8.1) — so trusting that getter turns every Bengali
  /// sentence the server composed into mojibake. Since Bengali is the default
  /// language, that is every screen.
  ///
  /// The backend now labels the charset as well. This stays because the
  /// client must not depend on it remembering to.
  static String _bodyText(http.Response response) =>
      utf8.decode(response.bodyBytes, allowMalformed: true);

  static Object? _decode(String body) =>
      body.isEmpty ? null : jsonDecode(body) as Object?;

  static Object? _decodeOrNull(String body) {
    try {
      return _decode(body);
    } on FormatException {
      return null;
    }
  }
}
