Feature: FF

  Scenario: First flaky
    Given flaky once

  Scenario: Second fails
    Given broken

  Scenario: Third never runs
    Given ok
