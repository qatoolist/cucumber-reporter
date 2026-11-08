Feature: Flaky only

  Scenario: Passes
    Given ok

  Scenario: Fails then passes
    Given attempt step c
