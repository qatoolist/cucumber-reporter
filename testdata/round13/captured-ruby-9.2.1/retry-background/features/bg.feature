Feature: Retry with failing background

  Background:
    Given the env is flaky per scenario

  Scenario: Bg flaky once
    Given bg target A
    Then it works

  Scenario: Bg always fails
    Given bg target B
    Then it works
