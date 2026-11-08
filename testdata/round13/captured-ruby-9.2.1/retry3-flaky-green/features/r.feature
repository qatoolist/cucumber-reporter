Feature: Flaky green

  Scenario: Fail fail pass
    Given step ffp

  Scenario: Pass first try
    Given step ok
