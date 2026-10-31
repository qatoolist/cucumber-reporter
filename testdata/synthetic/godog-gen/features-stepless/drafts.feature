Feature: Draft scenarios
  A scenario written before its steps.

  Scenario: Invoice is listed
    Given an invoice exists

  Scenario: No steps at all

  Scenario: Invoice is sent later
    Given an invoice exists
    When the invoice is sent
