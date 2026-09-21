import 'package:flutter/material.dart';

import '../session/session.dart';
import '../tokens/colors.dart';
import '../tokens/typography.dart';

/// The first frame, held only while stored tokens are read.
///
/// It waits on `Session.restore` rather than guessing: showing the sign-in
/// screen for the one frame before storage answers, to somebody who is already
/// signed in, reads as a bug.
class GoklaySplashScreen extends StatefulWidget {
  /// Creates the splash screen over [session].
  const GoklaySplashScreen({
    required this.session,
    required this.onReady,
    super.key,
  });

  /// Whose tokens to restore.
  final Session session;

  /// Called once storage has answered, with whether anybody is signed in.
  final void Function(bool isSignedIn) onReady;

  @override
  State<GoklaySplashScreen> createState() => _GoklaySplashScreenState();
}

class _GoklaySplashScreenState extends State<GoklaySplashScreen> {
  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) => _restore());
  }

  Future<void> _restore() async {
    await widget.session.restore();
    if (!mounted) {
      return;
    }
    widget.onReady(widget.session.isSignedIn);
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      backgroundColor: GoklayColors.brand,
      body: Center(
        child: Text(
          'GoKlay',
          style: GoklayTextStyles.display.copyWith(
            color: GoklayColors.onBrand,
          ),
        ),
      ),
    );
  }
}
