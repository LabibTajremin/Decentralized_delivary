import 'package:flutter/material.dart';
import 'package:goklay_core/goklay_core.dart';

/// The frame every screen in this app sits in.
///
/// One place for the app bar, the back button and the surface colour, so
/// twenty-odd screens cannot each drift a little from the design file.
class CustomerScaffold extends StatelessWidget {
  /// Creates a screen frame.
  const CustomerScaffold({
    required this.title,
    required this.body,
    this.actions = const <Widget>[],
    this.bottom,
    this.showBack = true,
    super.key,
  });

  /// The heading. Already in the user's language — either a chrome string or
  /// a name the server sent.
  final String title;

  /// The screen.
  final Widget body;

  /// Buttons at the end of the app bar.
  final List<Widget> actions;

  /// A bar pinned to the bottom — the checkout button, the cart summary.
  final Widget? bottom;

  /// Whether to draw a back button. False on the roots of the tab bar.
  final bool showBack;

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      backgroundColor: GoklayColors.surface,
      appBar: AppBar(
        automaticallyImplyLeading: showBack,
        title: Text(title, style: GoklayTextStyles.title),
        actions: actions,
      ),
      body: SafeArea(child: body),
      bottomNavigationBar: bottom == null
          ? null
          : SafeArea(
              child: Container(
                color: GoklayColors.surfaceRaised,
                padding: const EdgeInsets.all(GoklaySpacing.lg),
                child: bottom,
              ),
            ),
    );
  }
}

/// A heading over a group of rows.
class SectionHeading extends StatelessWidget {
  /// Creates a heading.
  const SectionHeading(this.text, {super.key});

  /// What it says.
  final String text;

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.fromLTRB(
        GoklaySpacing.lg,
        GoklaySpacing.lg,
        GoklaySpacing.lg,
        GoklaySpacing.sm,
      ),
      child: Text(text, style: GoklayTextStyles.emphasis),
    );
  }
}

/// A tappable row with a label, used all over the account screens.
class SettingRow extends StatelessWidget {
  /// Creates a row.
  const SettingRow({
    required this.label,
    required this.onTap,
    this.value,
    super.key,
  });

  /// What the row is.
  final String label;

  /// A value on the right, or null.
  final String? value;

  /// Called on tap. Null renders the row inert.
  final VoidCallback? onTap;

  @override
  Widget build(BuildContext context) {
    return GoklayTapTarget(
      onTap: onTap,
      semanticLabel: label,
      child: Container(
        color: GoklayColors.surfaceRaised,
        padding: const EdgeInsets.symmetric(
          horizontal: GoklaySpacing.lg,
          vertical: GoklaySpacing.md,
        ),
        child: Row(
          children: <Widget>[
            Expanded(child: Text(label, style: GoklayTextStyles.body)),
            if (value != null)
              Text(
                value!,
                style: GoklayTextStyles.caption.copyWith(
                  color: GoklayColors.textSecondary,
                ),
              ),
          ],
        ),
      ),
    );
  }
}

/// A text field with a label above it.
class LabelledField extends StatelessWidget {
  /// Creates a field.
  const LabelledField({
    required this.label,
    required this.controller,
    this.hint,
    this.keyboardType,
    this.maxLines = 1,
    super.key,
  });

  /// What the field is for.
  final String label;

  /// Holds the text.
  final TextEditingController controller;

  /// Placeholder, or null.
  final String? hint;

  /// Which keyboard to show.
  final TextInputType? keyboardType;

  /// How tall the field is.
  final int maxLines;

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: GoklaySpacing.sm),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: <Widget>[
          Text(
            label,
            style: GoklayTextStyles.caption.copyWith(
              color: GoklayColors.textSecondary,
            ),
          ),
          const SizedBox(height: GoklaySpacing.xxs),
          TextField(
            controller: controller,
            keyboardType: keyboardType,
            maxLines: maxLines,
            style: GoklayTextStyles.body,
            decoration: InputDecoration(hintText: hint),
          ),
        ],
      ),
    );
  }
}
