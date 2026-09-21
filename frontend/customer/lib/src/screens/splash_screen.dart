import 'package:flutter/material.dart';
import 'package:goklay_core/goklay_core.dart';

import '../app_scope.dart';
import '../dependencies.dart';

/// The first frame, held only while stored tokens are read.
///
/// It waits on `Session.restore` rather than guessing: showing the sign-in
/// screen for the one frame before storage answers, to somebody who is already
/// signed in, reads as a bug.
class SplashScreen extends StatefulWidget {
  /// Creates the splash screen.
  const SplashScreen({required this.onReady, super.key});

  /// Called once storage has answered, with whether anybody is signed in.
  final void Function(bool isSignedIn) onReady;

  @override
  State<SplashScreen> createState() => _SplashScreenState();
}

class _SplashScreenState extends State<SplashScreen> {
  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) => _restore());
  }

  Future<void> _restore() async {
    final Dependencies dependencies = AppScope.of(context);
    await dependencies.session.restore();
    if (!mounted) {
      return;
    }
    widget.onReady(dependencies.session.isSignedIn);
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
