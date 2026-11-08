Feature: Background failure

  Background:
    Given the database is up

  Scenario: First uses background
    Then it works

  Scenario: Second uses background
    Then it works
