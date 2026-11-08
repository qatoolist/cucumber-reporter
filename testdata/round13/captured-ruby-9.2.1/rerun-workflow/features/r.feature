Feature: Retry mixed

  Scenario: Fail fail pass
    Given step ffp

  Scenario: Fail then pending
    Given step fpend

  Scenario: Fail then undefined
    Given step fundef

  Scenario: Pass first try
    Given step ok

  Scenario: Always fails
    Given step always

  Scenario: Fail x3 then pass
    Given step fff_p
