-- Reverses nothing on purpose. Dropping last_totp_step would re-open the TOTP
-- replay #849 closes on a running server, and the column is harmless to an older
-- binary, which never reads it (GORM selects by struct fields). An operator who
-- wants it gone can drop it by hand once no binary that writes it is deployed.
SELECT 1;
