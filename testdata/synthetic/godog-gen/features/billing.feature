Feature: Billing
  Scenario: Invoice is pending implementation
    Given an invoice exists
    When the invoice is sent
    Then the customer receives it

  Scenario: Unknown step
    Given an invoice exists
    And something nobody implemented

  Scenario: Ambiguous step
    Given an ambiguous step
