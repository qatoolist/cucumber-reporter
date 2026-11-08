Feature: Checkout

  Background:
    Given the shop is open

  Scenario: Flaky payment
    When I pay
    Then the payment succeeds

  Scenario: Always fails
    When I pay
    Then the payment is declined

  Scenario Outline: Discounts
    When I apply code <code>
    Then the total is <total>

    Examples:
      | code | total |
      | A10  | 90    |
      | B20  | 80    |
