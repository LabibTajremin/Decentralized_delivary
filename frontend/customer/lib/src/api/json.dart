/// Readers for the JSON the backend sends.
///
/// Every one of them has a defined answer for a missing or wrongly typed
/// field, because the alternative is a screen that throws on a response it
/// half understands. A field the server stopped sending should leave a blank
/// row, not a crash report — the user is standing in a shop trying to pay.
///
/// They are deliberately not a code generator. The models here are small, and
/// generated files would have to be carved out of the 100% coverage gate.
library;

/// A string field, or `''`.
String readString(Map<String, Object?> json, String key) =>
    json[key] as String? ?? '';

/// A string field, or null when absent or empty.
String? readOptionalString(Map<String, Object?> json, String key) {
  final String value = readString(json, key);
  return value.isEmpty ? null : value;
}

/// An integer field, or `0`. Accepts a double, which some JSON encoders emit
/// for whole numbers.
int readInt(Map<String, Object?> json, String key) =>
    (json[key] as num?)?.toInt() ?? 0;

/// A double field, or `0`.
double readDouble(Map<String, Object?> json, String key) =>
    (json[key] as num?)?.toDouble() ?? 0;

/// A boolean field, or `false`.
///
/// False is the safe default for every flag this app reads: they are all
/// capabilities, and defaulting one to true would let a malformed response
/// enable an action the server never granted.
bool readBool(Map<String, Object?> json, String key) =>
    json[key] as bool? ?? false;

/// A nested object, or an empty map.
Map<String, Object?> readObject(Map<String, Object?> json, String key) {
  final Object? value = json[key];
  return value is Map<String, Object?> ? value : const <String, Object?>{};
}

/// A list of objects mapped through [parse], or an empty list.
List<T> readList<T>(
  Map<String, Object?> json,
  String key,
  T Function(Map<String, Object?>) parse,
) {
  final Object? value = json[key];
  if (value is! List<Object?>) {
    return <T>[];
  }
  return <T>[
    for (final Object? entry in value)
      if (entry is Map<String, Object?>) parse(entry),
  ];
}
