import 'package:flutter/material.dart';
import 'package:goklay_core/goklay_core.dart';

import '../l10n/customer_strings.dart';

/// The three pages the design draws before sign-in (`1:66`, `1:81`, `1:96`).
///
/// Their words are the only product copy in the app that is not either chrome
/// or server-composed, and they are here rather than in a response because
/// they are shown before any request is made — a first launch with no network
/// still has to say what this is.
class OnboardingScreen extends StatefulWidget {
  /// Creates the onboarding pager.
  const OnboardingScreen({required this.onFinished, super.key});

  /// Called when the customer skips or reaches the end.
  final VoidCallback onFinished;

  @override
  State<OnboardingScreen> createState() => _OnboardingScreenState();
}

class _OnboardingScreenState extends State<OnboardingScreen> {
  int _page = 0;

  static const int _pageCount = 3;

  void _next() {
    if (_page == _pageCount - 1) {
      widget.onFinished();
      return;
    }
    setState(() => _page += 1);
  }

  @override
  Widget build(BuildContext context) {
    final CustomerStrings strings = CustomerLocalizations.of(context);
    final List<(String, String)> pages = <(String, String)>[
      (strings.onboardingTitle1, strings.onboardingBody1),
      (strings.onboardingTitle2, strings.onboardingBody2),
      (strings.onboardingTitle3, strings.onboardingBody3),
    ];
    final (String title, String body) = pages[_page];
    final bool isLast = _page == _pageCount - 1;

    return Scaffold(
      backgroundColor: GoklayColors.surfaceRaised,
      body: SafeArea(
        child: Padding(
          padding: const EdgeInsets.all(GoklaySpacing.xl),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: <Widget>[
              Align(
                alignment: AlignmentDirectional.centerEnd,
                child: GoklayTapTarget(
                  onTap: widget.onFinished,
                  semanticLabel: strings.skip,
                  excludeChildSemantics: true,
                  child: Text(
                    strings.skip,
                    style: GoklayTextStyles.caption.copyWith(
                      color: GoklayColors.textSecondary,
                    ),
                  ),
                ),
              ),
              const Spacer(),
              Text(title, style: GoklayTextStyles.display),
              const SizedBox(height: GoklaySpacing.md),
              Text(body, style: GoklayTextStyles.body),
              const Spacer(),
              Row(
                mainAxisAlignment: MainAxisAlignment.center,
                children: <Widget>[
                  for (int index = 0; index < _pageCount; index++)
                    Container(
                      width: GoklaySpacing.sm,
                      height: GoklaySpacing.sm,
                      margin: const EdgeInsets.symmetric(
                        horizontal: GoklaySpacing.xxs,
                      ),
                      decoration: BoxDecoration(
                        shape: BoxShape.circle,
                        color: index == _page
                            ? GoklayColors.brand
                            : GoklayColors.brandSubtle,
                      ),
                    ),
                ],
              ),
              const SizedBox(height: GoklaySpacing.xl),
              GoklayButton(
                label: isLast ? strings.getStarted : strings.next,
                expand: true,
                onPressed: _next,
              ),
            ],
          ),
        ),
      ),
    );
  }
}
