@orders
Feature: Order processing
  Orders move from draft to confirmed.

  Scenario: Confirm a draft order
    Given a draft order with these lines:
      | sku   | qty |
      | A-100 | 2   |
      | B-200 | 1   |
    When the order is confirmed
    Then the confirmation message is:
      """application/json
      {"state": "confirmed"}
      """

  @slow
  Scenario Outline: Reject an order with quantity <qty>
    Given a draft order with quantity <qty>
    When the order is confirmed
    Then the order is rejected

    Examples: invalid quantities
      | qty |
      | 0   |
      | -1  |
