// Package seatlayer is the official Go server SDK for SeatLayer reserved
// seating: seating charts, seat maps, events, live inventory, temporary holds,
// server-confirmed booking and webhook verification, with idempotency and
// retries built in.
//
// SeatLayer is interactive seating chart software built for stadium scale.
// Platforms embed the white-label seat picker with their own checkout;
// organizers sell seated events on their own website with their own payment
// gateway. This package is the trusted server side of that integration: the
// browser or app selects seats and creates a hold, and your server uses this
// client to inspect the hold, confirm the booking after payment and verify
// signed webhooks.
//
// Quickstart: https://docs.seatlayer.io/start/quickstart/
// Holds and checkout: https://docs.seatlayer.io/buyer-sdk/holds-and-checkout/
// API reference: https://docs.seatlayer.io/openapi.json
package seatlayer
