Feature: Rules with backgrounds

  Background:
    Given feature background

  Rule: Good rule
    Background:
      Given rule background ok

    Example: In good rule
      Then pass

  Rule: Bad rule
    Background:
      Given rule background fails

    Example: In bad rule one
      Then pass

    Example: In bad rule two
      Then pass

  Rule: No background rule
    Example: Plain rule example fails
      Then fail
