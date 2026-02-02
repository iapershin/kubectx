#!/usr/bin/env bats

COMMAND="${COMMAND:-$BATS_TEST_DIRNAME/../kubectx}"

load common

@test "--help should not fail" {
  run ${COMMAND} --help
  echo "$output"
  [ "$status" -eq 0 ]
}

@test "-h should not fail" {
  run ${COMMAND} -h
  echo "$output"
  [ "$status" -eq 0 ]
}

@test "switch to previous context when no one exists" {
  use_config config1

  run ${COMMAND} -
  echo "$output"
  [ "$status" -eq 1 ]
  [[ $output = *"no previous context found" ]]
}

@test "list contexts when no kubeconfig exists" {
  run ${COMMAND}
  echo "$output"
  [ "$status" -eq 0 ]
  [[ "$output" = "warning: kubeconfig file not found" ]]
}

@test "get one context and list contexts" {
  use_config config1

  run ${COMMAND}
  echo "$output"
  [ "$status" -eq 0 ]
  [[ "$output" = "user1@cluster1" ]]
}

@test "get two contexts and list contexts" {
  use_config config2

  run ${COMMAND}
  echo "$output"
  [ "$status" -eq 0 ]
  [[ "$output" = *"user1@cluster1"* ]]
  [[ "$output" = *"user2@cluster1"* ]]
}

@test "get two contexts and select contexts" {
  use_config config2

  run ${COMMAND} user1@cluster1
  echo "$output"
  [ "$status" -eq 0 ]
  echo "$(get_context)"
  [[ "$(get_context)" = "user1@cluster1" ]]

  run ${COMMAND} user2@cluster1
  echo "$output"
  [ "$status" -eq 0 ]
  echo "$(get_context)"
  [[ "$(get_context)" = "user2@cluster1" ]]
}

@test "get two contexts and switch between contexts" {
  use_config config2

  run ${COMMAND} user1@cluster1
  echo "$output"
  [ "$status" -eq 0 ]
  echo "$(get_context)"
  [[ "$(get_context)" = "user1@cluster1" ]]

  run ${COMMAND} user2@cluster1
  echo "$output"
  [ "$status" -eq 0 ]
  echo "$(get_context)"
  [[ "$(get_context)" = "user2@cluster1" ]]

  run ${COMMAND} -
  echo "$output"
  [ "$status" -eq 0 ]
  echo "$(get_context)"
  [[ "$(get_context)" = "user1@cluster1" ]]

  run ${COMMAND} -
  echo "$output"
  [ "$status" -eq 0 ]
  echo "$(get_context)"
  [[ "$(get_context)" = "user2@cluster1" ]]
}

@test "get one context and switch to non existent context" {
  use_config config1

  run ${COMMAND} "unknown-context"
  echo "$output"
  [ "$status" -eq 1 ]
}

@test "-c/--current fails when no context set" {
  use_config config1

  run "${COMMAND}" -c
  echo "$output"
  [ $status -eq 1 ]
  run "${COMMAND}" --current
  echo "$output"
  [ $status -eq 1 ]
}

@test "-c/--current prints the current context" {
  use_config config1

  run "${COMMAND}" user1@cluster1
  [ $status -eq 0 ]

  run "${COMMAND}" -c
  echo "$output"
  [ $status -eq 0 ]
  [[ "$output" = "user1@cluster1" ]]
  run "${COMMAND}" --current
  echo "$output"
  [ $status -eq 0 ]
  [[ "$output" = "user1@cluster1" ]]
}

@test "rename context" {
  use_config config2

  run ${COMMAND} "new-context=user1@cluster1"
  echo "$output"
  [ "$status" -eq 0 ]

  run ${COMMAND}
  echo "$output"
  [ "$status" -eq 0 ]
  [[ ! "$output" = *"user1@cluster1"* ]]
  [[ "$output" = *"new-context"* ]]
  [[ "$output" = *"user2@cluster1"* ]]
}

@test "rename current context" {
  use_config config2

  run ${COMMAND} user2@cluster1
  echo "$output"
  [ "$status" -eq 0 ]

  run ${COMMAND} new-context=.
  echo "$output"
  [ "$status" -eq 0 ]

  run ${COMMAND}
  echo "$output"
  [ "$status" -eq 0 ]
  [[ ! "$output" = *"user2@cluster1"* ]]
  [[ "$output" = *"user1@cluster1"* ]]
  [[ "$output" = *"new-context"* ]]
}

@test "delete context" {
  use_config config2

  run ${COMMAND} -d "user1@cluster1"
  echo "$output"
  [ "$status" -eq 0 ]

  run ${COMMAND}
  echo "$output"
  [ "$status" -eq 0 ]
  [[ ! "$output" = "user1@cluster1" ]]
  [[ "$output" = "user2@cluster1" ]]
}

@test "delete current context" {
  use_config config2

  run ${COMMAND} user2@cluster1
  echo "$output"
  [ "$status" -eq 0 ]

  run ${COMMAND} -d .
  echo "$output"
  [ "$status" -eq 0 ]

  run ${COMMAND}
  echo "$output"
  [ "$status" -eq 0 ]
  [[ ! "$output" = "user2@cluster1" ]]
  [[ "$output" = "user1@cluster1" ]]
}

@test "delete non existent context" {
  use_config config1

  run ${COMMAND} -d "unknown-context"
  echo "$output"
  [ "$status" -eq 1 ]
}

@test "delete several contexts" {
  use_config config2

  run ${COMMAND} -d "user1@cluster1" "user2@cluster1"
  echo "$output"
  [ "$status" -eq 0 ]

  run ${COMMAND}
  echo "$output"
  [ "$status" -eq 0 ]
  [[ "$output" = "" ]]
}

@test "delete several contexts including a non existent one" {
  use_config config2

  run ${COMMAND} -d "user1@cluster1" "non-existent" "user2@cluster1"
  echo "$output"
  [ "$status" -eq 1 ]

  run ${COMMAND}
  echo "$output"
  [ "$status" -eq 0 ]
  [[ "$output" = "user2@cluster1" ]]
}

@test "unset selected context" {
  use_config config2

  run ${COMMAND} user1@cluster1
  [ "$status" -eq 0 ]

  run ${COMMAND} -u
  [ "$status" -eq 0 ]

  run ${COMMAND} -c
  [ "$status" -ne 0 ]
}

@test "switch context with namespace using -n flag" {
  use_config config2
  export _MOCK_NAMESPACES=1

  run ${COMMAND} user1@cluster1 -n ns1
  echo "$output"
  [ "$status" -eq 0 ]
  [[ "$output" = *'Switched to context "user1@cluster1" and namespace "ns1"'* ]]
  echo "$(get_context)"
  [[ "$(get_context)" = "user1@cluster1" ]]
  echo "$(get_namespace)"
  [[ "$(get_namespace)" = "ns1" ]]
}

@test "switch context with namespace using --namespace flag" {
  use_config config2
  export _MOCK_NAMESPACES=1

  run ${COMMAND} user1@cluster1 --namespace ns2
  echo "$output"
  [ "$status" -eq 0 ]
  [[ "$output" = *'Switched to context "user1@cluster1" and namespace "ns2"'* ]]
  echo "$(get_context)"
  [[ "$(get_context)" = "user1@cluster1" ]]
  echo "$(get_namespace)"
  [[ "$(get_namespace)" = "ns2" ]]
}

@test "switch context with namespace flag before context name" {
  use_config config2
  export _MOCK_NAMESPACES=1

  run ${COMMAND} -n ns1 user2@cluster1
  echo "$output"
  [ "$status" -eq 0 ]
  [[ "$output" = *'Switched to context "user2@cluster1" and namespace "ns1"'* ]]
  echo "$(get_context)"
  [[ "$(get_context)" = "user2@cluster1" ]]
  echo "$(get_namespace)"
  [[ "$(get_namespace)" = "ns1" ]]
}

@test "switch context with non-existing namespace" {
  use_config config2
  export _MOCK_NAMESPACES=1

  run ${COMMAND} user1@cluster1 -n unknown-ns
  echo "$output"
  [ "$status" -eq 1 ]
  [[ "$output" = *'no namespace exists with name "unknown-ns"'* ]]
}

@test "switch context with namespace and then switch to another context" {
  use_config config2
  export _MOCK_NAMESPACES=1

  run ${COMMAND} user1@cluster1 -n ns1
  [ "$status" -eq 0 ]
  [[ "$(get_context)" = "user1@cluster1" ]]
  [[ "$(get_namespace)" = "ns1" ]]

  run ${COMMAND} user2@cluster1 -n ns2
  [ "$status" -eq 0 ]
  [[ "$(get_context)" = "user2@cluster1" ]]
  [[ "$(get_namespace)" = "ns2" ]]
}

@test "-n flag without namespace value should fail" {
  use_config config2

  run ${COMMAND} -n
  echo "$output"
  [ "$status" -eq 1 ]
  [[ "$output" = *"'-n' requires a namespace argument"* ]]
}

@test "-n flag without context name should fail" {
  use_config config2

  run ${COMMAND} -n ns1
  echo "$output"
  [ "$status" -eq 1 ]
  [[ "$output" = *"context name is required when using -n flag"* ]]
}
