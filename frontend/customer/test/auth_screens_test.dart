import 'package:flutter_test/flutter_test.dart';
import 'package:goklay_core/goklay_core.dart';
import 'package:goklay_customer/src/dependencies.dart';
import 'package:goklay_customer/src/l10n/customer_strings.dart';
import 'package:goklay_customer/src/screens/onboarding_screen.dart';
import 'package:goklay_customer/src/screens/sign_in_options_screen.dart';

import 'support/harness.dart';

void main() {
  const CustomerStringsBn bn = CustomerStringsBn();
  const GoklayStringsBn core = GoklayStringsBn();
  late FakeBackend backend;

  setUp(() => backend = FakeBackend(<String, Object? Function(SentRequest)>{}));

  group('onboarding', () {
    testWidgets('walks the three pages and then finishes', (
      WidgetTester tester,
    ) async {
      final Dependencies dependencies = await harnessDependencies(backend);
      int finished = 0;
      await pumpScreen(
        tester,
        OnboardingScreen(onFinished: () => finished += 1),
        dependencies: dependencies,
      );
      expect(find.text(bn.onboardingTitle1), findsOneWidget);
      await tester.tap(find.bySemanticsLabel(bn.next));
      await tester.pumpAndSettle();
      expect(find.text(bn.onboardingTitle2), findsOneWidget);
      await tester.tap(find.bySemanticsLabel(bn.next));
      await tester.pumpAndSettle();
      expect(find.text(bn.onboardingTitle3), findsOneWidget);
      await tester.tap(find.bySemanticsLabel(core.getStarted));
      await tester.pumpAndSettle();
      expect(finished, 1);
      dependencies.dispose();
    });

    testWidgets('skipping finishes straight away', (
      WidgetTester tester,
    ) async {
      final Dependencies dependencies = await harnessDependencies(backend);
      int finished = 0;
      await pumpScreen(
        tester,
        OnboardingScreen(onFinished: () => finished += 1),
        dependencies: dependencies,
      );
      await tester.tap(find.bySemanticsLabel(bn.skip));
      await tester.pumpAndSettle();
      expect(finished, 1);
      dependencies.dispose();
    });
  });

  testWidgets('sign-in offers the one method this product has', (
    WidgetTester tester,
  ) async {
    final Dependencies dependencies = await harnessDependencies(backend);
    int continued = 0;
    await pumpScreen(
      tester,
      SignInOptionsScreen(onContinue: () => continued += 1),
      dependencies: dependencies,
    );
    expect(find.byType(GoklayButton), findsOneWidget);
    await tester.tap(find.bySemanticsLabel(bn.continueWithPhone));
    await tester.pumpAndSettle();
    expect(continued, 1);
    dependencies.dispose();
  });

  testWidgets('sign-in offers the one method this product has', (
    WidgetTester tester,
  ) async {
    final Dependencies dependencies = await harnessDependencies(backend);
    int continued = 0;
    await pumpScreen(
      tester,
      SignInOptionsScreen(onContinue: () => continued += 1),
      dependencies: dependencies,
    );
    expect(find.byType(GoklayButton), findsOneWidget);
    await tester.tap(find.bySemanticsLabel(bn.continueWithPhone));
    await tester.pumpAndSettle();
    expect(continued, 1);
    dependencies.dispose();
  });

}
